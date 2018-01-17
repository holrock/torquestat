# torquestat

Torque status viewer.

torquestat requires pbsnodes and qstat command.

## install

[download linux binary](https://github.com/holrock/torquestat/releases)

## run

```
./torquestat
curl http://localhost:8111
```

or

```
./torquestat -port 8080 -pbsnodes /usr/local/bin/pbsnodes -qstat /usr/local/bin/qstat
```

## build

```
make build_iamge
make docker
```
