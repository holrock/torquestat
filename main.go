package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type LsfJob struct {
	JobID      string
	Name       string
	Owner      string
	State      string
	Queue      string
	FromHost   string
	ExecHost   string
	SubmitTime string
}

type LsfHost struct {
	HostName       string
	Status         string
	JobLimtPerUser int
	Max            int
	NJobs          int
	Run            int
	SSUSP          int
	USUSP          int
	RSV            int
}

type LsfLoad struct {
	HostName string
	Status   string
	R15s     string
	R1m      string
	R15m     string
	UT       string
	PG       string
	LS       string
	IT       string
	Tmp      string
	Swp      string
	Mem      string
}

type LsfNode struct {
	HostName string
	Status   string
	UT       string
	Mem      string
	Max      int
	NJobs    int
}

type LsfNodes struct {
	NodeList   []LsfNode
	TotalCores int
	DownCores  int
	TotalJobs  int
	TotalMem   int
	LastUpdate string
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

var jobStatusMap = map[string]string{
	"PEND":  "PENDING",
	"PSUSP": "PSUSP",
	"RUN":   "RUN",
	"USUSP": "USUSP",
	"SSUSP": "SSUSP",
	"DONE":  "DONE",
	"EXIT":  "EXIT",
	"ZOMBI": "ZOMBI",
	"UNKWN": "UNKNOWN",
}

func parseLsload(s string) []LsfLoad {
	var acc []LsfLoad

	for _, line := range strings.Split(s, "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 12 {
			continue
		}

		h := LsfLoad{
			HostName: fs[0],
			Status:   fs[1],
			R15s:     fs[2],
			R1m:      fs[3],
			R15m:     fs[4],
			UT:       fs[5],
			PG:       fs[6],
			LS:       fs[7],
			IT:       fs[8],
			Tmp:      fs[9],
			Swp:      fs[10],
			Mem:      fs[11],
		}
		acc = append(acc, h)
	}
	return acc
}

func parseBhosts(s string) []LsfHost {
	var acc []LsfHost
	for _, line := range strings.Split(s, "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 9 {
			continue
		}

		h := LsfHost{
			HostName:       fs[0],
			Status:         fs[1],
			JobLimtPerUser: atoi(fs[2]),
			Max:            atoi(fs[3]),
			NJobs:          atoi(fs[4]),
			Run:            atoi(fs[5]),
			SSUSP:          atoi(fs[6]),
			USUSP:          atoi(fs[7]),
			RSV:            atoi(fs[8]),
		}
		acc = append(acc, h)
	}
	return acc
}

func parseBjobs(s string) []LsfJob {
	var acc []LsfJob
	for _, line := range strings.Split(s, "\n")[1:] {
		fs := strings.Fields(line)
		if len(fs) != 10 {
			continue
		}
		j := LsfJob{
			JobID:      fs[0],
			Owner:      fs[1],
			State:      fs[2],
			Queue:      fs[3],
			FromHost:   fs[4],
			ExecHost:   fs[5],
			Name:       fs[6],
			SubmitTime: fs[7] + " " + fs[8] + " " + fs[9],
		}
		acc = append(acc, j)
	}
	return acc
}

func mergeLSFNode(hosts []LsfHost, loads []LsfLoad) []LsfNode {
	var acc []LsfNode
	d := make(map[string]LsfLoad, len(loads))
	for _, load := range loads {
		d[load.HostName] = load
	}
	for _, host := range hosts {
		load, ok := d[host.HostName]

		if ok {
			acc = append(acc, LsfNode{
				HostName: host.HostName,
				Status:   load.Status,
				UT:       load.UT,
				Mem:      load.Mem,
				Max:      host.Max,
				NJobs:    host.NJobs,
			})
		} else {
			acc = append(acc, LsfNode{
				HostName: host.HostName,
				Max:      host.Max,
				NJobs:    host.NJobs,
			})
		}
	}
	return acc
}

func (n *LsfNode) StateColor() string {
	switch n.Status {
	case "ok":
		return "success"
	case "busy":
		return "warning"

	case "unavail":
		return "error"

	default:
		return "error"

	}
}

func (n *LsfNode) URL() string {
	return "/node/" + n.HostName
}

func (n *LsfJob) URL() string {
	return "/job/" + n.JobID
}

func (j *LsfJob) LongState() string {
	return jobStatusMap[j.State]
}

func (j *LsfJob) StateColor() string {
	switch j.State {
	case "RUN":
		return "success"
	case "PEND":
		return "warning"
	case "DONE":
		return "gray"
	case "EXIT":
		return "gray"
	case "PSUSP":
		return "gray"
	case "USUSP":
		return "gray"
	case "SSUSP":
		return "gray"
	default:
		return "error"
	}
}

func lava() ([]LsfJob, []LsfHost, []LsfLoad) {
	f, err := os.Open("testdata/bjobs.txt")
	if err != nil {
		log.Fatal(err)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	s := string(content)
	lsf_jobs := parseBjobs(s)

	f, err = os.Open("testdata/bhosts.txt")
	if err != nil {
		log.Fatal(err)
	}
	content, err = io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	lsf_hosts := parseBhosts(string(content))

	f, err = os.Open("testdata/lsload.txt")
	if err != nil {
		log.Fatal(err)
	}
	content, err = io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	lsf_loads := parseLsload(string(content))
	return lsf_jobs, lsf_hosts, lsf_loads
}

var JobStatusMap = map[string]string{
	"C": "completed",
	"E": "exiting",
	"H": "held",
	"Q": "queued",
	"R": "running",
	"T": "moving",
	"W": "waiting",
}

func pbsnodes(pbsnodesCmd string, w http.ResponseWriter, templ *template.Template) {
	_, lsf_hosts, lsf_loads := lava()
	lsfNodes := LsfNodes{
		NodeList: mergeLSFNode(lsf_hosts, lsf_loads),
	}
	nJobs := 0
	nCPU := 0
	for _, n := range lsfNodes.NodeList {
		nJobs += n.NJobs
		nCPU += n.Max
	}
	lsfNodes.TotalJobs = nJobs
	lsfNodes.TotalCores = nCPU
	lsfNodes.LastUpdate = time.Now().Format("2006-1-2 15:04:05")
	/*
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
	*/
	b := new(bytes.Buffer)

	err := templ.ExecuteTemplate(b, "pbsnodes.html", lsfNodes)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

func qstatJoblist(qstatCmd string, w http.ResponseWriter, templ *template.Template) {
	lsf_jobs, _, _ := lava()

	b := new(bytes.Buffer)
	joblist := struct {
		JobList []LsfJob
	}{
		JobList: lsf_jobs,
	}
	err := templ.ExecuteTemplate(b, "qstat_joblist.html", joblist)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	io.WriteString(w, b.String())
}

/*
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
*/
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
		b, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
		name := filepath.Base(fname)
		nt := t.New(name)
		_, err = nt.Parse(string(b))
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	return t
}

func startServer(port int, pbsnodesCmd string, qstatCmd string) {
	t := initTemplate()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pbsnodes(pbsnodesCmd, w, t)
	})

	http.HandleFunc("/job", func(w http.ResponseWriter, r *http.Request) {
		qstatJoblist(qstatCmd, w, t)
	})
	/*
		var jobParamRegex = regexp.MustCompile(`\A/job/([^/]+)`)
		http.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
			m := jobParamRegex.FindStringSubmatch(r.URL.Path)
			if len(m) == 2 {
				if strings.Contains(m[1], "[]") {
					qstatJoblist(qstatCmd, w, m[1], t)
				} else {
					// qstatJob(qstatCmd, w, m[1], t)
				}
			} else {
				qstatJoblist(qstatCmd, w, "", t)
			}
		})
	*/
	http.Handle("/css/", http.FileServer(Assets))
	http.Handle("/js/", http.FileServer(Assets))

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
