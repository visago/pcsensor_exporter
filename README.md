# pcsensor_exporter

Exports pcsensor.com sensors as prometheus metrics

```
$ curl http://127.0.0.1:9876/probe?target=192.168.3.10

# HELP probe_duration_seconds Returns how long the probe took to complete in seconds
# TYPE probe_duration_seconds gauge
probe_duration_seconds 0.027034803
# HELP probe_pcsensors_temperature_celcius Temperature detected by pcsensors probe in celcius
# TYPE probe_pcsensors_temperature_celcius gauge
probe_pcsensors_temperature_celcius{probe="T1"} 24.75
probe_pcsensors_temperature_celcius{probe="T2"} 24.5
# HELP probe_success Displays whether or not the probe was a success
# TYPE probe_success gauge
probe_success 1
```

## Building
```
go get
GOOS=linux GOARCH=mips64 go build
```