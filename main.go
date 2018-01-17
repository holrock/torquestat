package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var JobStatusMap = map[string]string{
	"C": "completed",
	"E": "exiting",
	"H": "held",
	"Q": "queued",
	"R": "running",
	"T": "moving",
	"W": "waiting",
}

type QstatJobs struct {
	Root    xml.Name `xml:"Data"`
	JobList []Job    `xml:"Job"`
}

type Job struct {
	JobID        string `xml:"Job_Id"`
	Name         string `xml:"Job_Name"`
	Owner        string `xml:"Job_Owner"`
	State        string `xml:"job_state"`
	Queue        string `xml:"queue"`
	ExecHost     string `xml:"exec_host"`
	WallTime     string `xml:"resources_used>walltime"`
	Mem          string `xml:"resources_used>mem"`
	ArrayRequest string `xml:"job_array_request"`
}

type QstatJobMap struct {
	ResourcesUsed map[string]string
	VariableList  map[string]string
	Elems         map[string]string
}

type PbsNodes struct {
	NodeList   []*Node `xml:"Node"`
	TotalCores int
	DownCores  int
	TotalJobs  int
	TotalMem   int
}

type Node struct {
	Jobs       string `xml:"jobs"`
	JobList    []string
	Name       string `xml:"name"`
	NumJobs    int
	NumProcs   int    `xml:"np"`
	PowerState string `xml:"power_state"`
	State      string `xml:"state"`
	Status     map[string]string
	StatusStr  string `xml:"status"`
}

func (p *PbsNodes) AvailCores() int {
	return p.TotalCores - p.DownCores
}

func xmlToPbsNodes(content []byte) (*PbsNodes, error) {
	pbsnodes := new(PbsNodes)
	err := xml.Unmarshal(content, &pbsnodes)
	if err != nil {
		return nil, err
	}
	for _, n := range pbsnodes.NodeList {
		n.parseStatus()
		pbsnodes.TotalCores += n.NumProcs
		pbsnodes.TotalMem += n.GetGiBMem("physmem")
		if n.State == "down" || n.State == "offline" {
			pbsnodes.DownCores += n.NumProcs
		}
		if n.Jobs != "" {
			n.JobList = strings.Split(n.Jobs, ",")
			n.NumJobs = len(n.JobList)
			pbsnodes.TotalJobs += n.NumJobs
		}
	}
	return pbsnodes, nil
}

func (n *Node) parseStatus() {
	n.Status = make(map[string]string)
  if n.StatusStr == "" {
    return
  }
	for _, s := range strings.Split(n.StatusStr, ",") {
		v := strings.Split(s, "=")
		n.Status[v[0]] = v[1]
	}
}

func (n *Node) GetGiBMem(key string) int {
	mem, err := strconv.Atoi(strings.Replace(n.Status[key], "kb", "", -1))
	if err != nil {
		return 0
	}
	return mem / (1024 * 1024)
}

func (n *Node) URL() string {
	return "/node/" + n.Name
}

var nodeStateRegex = regexp.MustCompile("down|offline|unknown")

func (n *Node) StateColor() string {
	if nodeStateRegex.MatchString(n.State) {
		return "error"
	}
	if strings.Contains(n.State, "job-exclusive") {
		return "warning"
	}
	return "success"
}

func (j *Job) UnifiedID() string {
	if len(j.ArrayRequest) == 0 {
		return j.JobID
	}
	return strings.Replace(j.JobID, "[]", "["+j.ArrayRequest+"]", 1)
}

func (j *Job) URL() string {
	return "/job/" + j.JobID
}

func (j *Job) LongState() string {
	return JobStatusMap[j.State]
}

func (j *Job) StateColor() string {
	switch j.State {
	case "R":
		return "success"
	case "Q":
		return "warning"
	case "C":
		return "gray"
	case "E":
		return "gray"
	default:
		return "error"
	}
}

func xmlToQstatJobs(content []byte) (*QstatJobs, error) {
	jobs := new(QstatJobs)
	err := xml.Unmarshal(content, &jobs)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (qj *QstatJobMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if start.Name.Local == "Data" || start.Name.Local == "Job" {
		token, err := d.Token()
		if err != nil {
			return err
		}
		t := token.(xml.StartElement)
		if err := d.DecodeElement(qj, &t); err != nil {
			return err
		}
	}

	tag := start.Name.Local
	currentMap := qj.Elems
	for {
		token, err := d.Token()
		if token == nil {
			break
		}
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			t := token.(xml.StartElement)
			if t.Name.Local == "resources_used" {
				currentMap = qj.ResourcesUsed
			}
			tag = t.Name.Local
		case xml.EndElement:
			if token.(xml.EndElement).Name.Local == "resources_used" {
				currentMap = qj.Elems
			}
		case xml.CharData:
			s := string(token.(xml.CharData))
			if tag == "Variable_List" {
				for _, s := range strings.Split(s, ",") {
					v := strings.Split(s, "=")
					qj.VariableList[v[0]] = v[1]
				}
			} else {
				currentMap[tag] = s
			}
		}
	}
	return nil
}

func pbsnodes(pbsnodesCmd string, w http.ResponseWriter, templ *template.Template) {
	content, err := exec.Command(pbsnodesCmd, "-x").Output()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	pbsnodes, err := xmlToPbsNodes(content)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	b := new(bytes.Buffer)

	err = templ.ExecuteTemplate(b, "pbsnodes.html", pbsnodes)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

var nodeParamRegex = regexp.MustCompile("\\A/node/([^/]+)")
var nodeParamValidationRegex = regexp.MustCompile("\\A[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*\\z")

func pbsnodesNode(pbsnodesCmd string, w http.ResponseWriter, r *http.Request, templ *template.Template) {
	m := nodeParamRegex.FindStringSubmatch(r.URL.Path)
	if len(m) != 2 {
		http.Error(w, "invalid parameter", 400)
		return
	}
	if !nodeParamValidationRegex.MatchString(m[1]) {
		http.Error(w, "invalid parameter", 400)
		return
	}
	content, err := exec.Command(pbsnodesCmd, "-ax", m[1]).Output()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	pbsnodes, err := xmlToPbsNodes(content)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	b := new(bytes.Buffer)
	err = templ.ExecuteTemplate(b, "pbsnodes_node.html", pbsnodes.NodeList[0])
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

var jobIDValidationRegex = regexp.MustCompile("\\A\\d+(?:\\[\\d*\\])?\\.[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*\\z")

func execQstat(qstatCmd string, arrayid string) ([]byte, error) {
	if len(arrayid) == 0 {
		return exec.Command(qstatCmd, "-x").Output()
	}
	if !jobIDValidationRegex.MatchString(arrayid) {
		return nil, errors.New("invalid parameter")
	}
	return exec.Command(qstatCmd, "-xt", arrayid).Output()
}

func qstatJoblist(qstatCmd string, w http.ResponseWriter, arrayid string, templ *template.Template) {
	content, err := execQstat(qstatCmd, arrayid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	joblist, err := xmlToQstatJobs(content)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	b := new(bytes.Buffer)
	err = templ.ExecuteTemplate(b, "qstat_joblist.html", joblist)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

func qstatJob(qstatCmd string, w http.ResponseWriter, jobid string, templ *template.Template) {
	if !jobIDValidationRegex.MatchString(jobid) {
		http.Error(w, "invalid parameter", 400)
		return
	}
	content, err := exec.Command(qstatCmd, "-x", jobid).Output()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	job := QstatJobMap{}
	job.ResourcesUsed = make(map[string]string)
	job.VariableList = make(map[string]string)
	job.Elems = make(map[string]string)
	xml.Unmarshal(content, &job)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	b := new(bytes.Buffer)
	err = templ.ExecuteTemplate(b, "qstat_job.html", job)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

func initTemplate() *template.Template {

	f := template.FuncMap{
		"add":  func(x int, y int) int { return x + y },
		"link": func(s string) string { return s },
	}
	t := template.New("t").Funcs(f)

	htmls := []string{
		"/template/pbsnodes.html",
		"/template/pbsnodes_node.html",
		"/template/qstat_joblist.html",
		"/template/qstat_job.html",
	}
	for _, fname := range htmls {
		f, err := Assets.Open(fname)
		if err != nil {
			panic(err.Error())
		}
		b, err := ioutil.ReadAll(f)
		name := filepath.Base(fname)
		nt := t.New(name)
		_, err = nt.Parse(string(b))
		if err != nil {
			panic(err.Error())
		}
	}
	return t
}

func startServer(port int, pbsnodesCmd string, qstatCmd string) {
	t := initTemplate()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pbsnodes(pbsnodesCmd, w, t)
	})

	http.HandleFunc("/node/", func(w http.ResponseWriter, r *http.Request) {
		pbsnodesNode(pbsnodesCmd, w, r, t)
	})

	http.HandleFunc("/job", func(w http.ResponseWriter, r *http.Request) {
		qstatJoblist(qstatCmd, w, "", t)
	})

	var jobParamRegex = regexp.MustCompile("\\A/job/([^/]+)")
	http.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
		m := jobParamRegex.FindStringSubmatch(r.URL.Path)
		if len(m) == 2 {
			if strings.Contains(m[1], "[]") {
				qstatJoblist(qstatCmd, w, m[1], t)
			} else {
				qstatJob(qstatCmd, w, m[1], t)
			}
		} else {
			qstatJoblist(qstatCmd, w, "", t)
		}
	})

	http.Handle("/css/", http.FileServer(Assets))

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("Listen:", err)
	}
}

func main() {
	var (
		port        = flag.Int("port", 8111, "http port")
		pbsnodesCmd = flag.String("pbsnodes", "/usr/bin/pbsnodes", "pbsnodes command path")
		qstatCmd    = flag.String("qstat", "/usr/bin/qstat", "qstat command path")
	)
	flag.Parse()

	startServer(*port, *pbsnodesCmd, *qstatCmd)
}
