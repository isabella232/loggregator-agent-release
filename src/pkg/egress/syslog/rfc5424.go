package syslog

import (
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

const RFC5424TimeOffsetNum = "2006-01-02T15:04:05.999999-07:00"

// gaugeStructuredDataID contains the registered enterprise ID for the Cloud
// Foundry Foundation.
// See: https://www.iana.org/assignments/enterprise-numbers/enterprise-numbers
const (
	gaugeStructuredDataID   = "gauge@47450"
	timerStructuredDataID   = "timer@47450"
	counterStructuredDataID = "counter@47450"
	eventStructuredDataID = "event@47450"
)

func ToRFC5424(env *loggregator_v2.Envelope, hostname string) ([][]byte, error) {
	if len(hostname) > 255 {
		return nil, invalidValue("Hostname", hostname)
	}

	appID := env.GetSourceId()
	if len(appID) > 48 {
		return nil, invalidValue("AppName", appID)
	}

	if len(env.InstanceId) > 128 {
		return nil, invalidValue("AppName", appID)
	}

	switch env.GetMessage().(type) {
	case *loggregator_v2.Envelope_Log:
		return [][]byte{
			toRFC5424LogMessage(env, hostname, appID),
		}, nil
	case *loggregator_v2.Envelope_Gauge:
		return toRFC5424GaugeMessage(env, hostname, appID), nil
	case *loggregator_v2.Envelope_Timer:
		return [][]byte{
			toRFC5424TimerMessage(env, hostname, appID),
		}, nil
	case *loggregator_v2.Envelope_Counter:
		return [][]byte{
			toRFC5424CounterMessage(env, hostname, appID),
		}, nil
	case *loggregator_v2.Envelope_Event:
		return [][]byte{
			toRFC5424EventMessage(env, hostname, appID),
		}, nil
	default:
		return nil, nil
	}
}

func invalidValue(property, value string) error {
	return fmt.Errorf("Invalid value \"%s\" for property %s \n", value, property)
}

func toRFC5424CounterMessage(env *loggregator_v2.Envelope, hostname, appID string) []byte {
	counter := env.GetCounter()
	sd := `[` + counterStructuredDataID + ` name="` + counter.GetName() + `" total="` + strconv.FormatUint(counter.GetTotal(), 10) + `" delta="` + strconv.FormatUint(counter.GetDelta(), 10) + `"]`

	return toRFC5424MetricMessage(env, hostname, appID, sd)
}

func toRFC5424GaugeMessage(env *loggregator_v2.Envelope, hostname, appID string) [][]byte {
	gauges := make([][]byte, 0, 5)

	for name, g := range env.GetGauge().GetMetrics() {
		sd := `[` + gaugeStructuredDataID + ` name="` + name + `" value="` + strconv.FormatFloat(g.GetValue(), 'g', -1, 64) + `" unit="` + g.GetUnit() + `"]`
		gauges = append(gauges, toRFC5424MetricMessage(env, hostname, appID, sd))
	}

	return gauges
}

func toRFC5424TimerMessage(env *loggregator_v2.Envelope, hostname, appID string) []byte {
	timer := env.GetTimer()
	sd := fmt.Sprintf(`[%s name="%s" start="%d" stop="%d"]`, timerStructuredDataID, timer.GetName(), timer.GetStart(), timer.GetStop())

	return toRFC5424MetricMessage(env, hostname, appID, sd)
}

func toRFC5424EventMessage(env *loggregator_v2.Envelope, hostname, appID string) []byte {
	event := env.GetEvent()
	sd := fmt.Sprintf(`[%s title="%s" body="%s"]`, eventStructuredDataID, event.GetTitle(), event.GetBody())

	return toRFC5424MetricMessage(env, hostname, appID, sd)
}

func toRFC5424LogMessage(env *loggregator_v2.Envelope, hostname, appID string) []byte {
	priority := genPriority(env.GetLog().Type)
	ts := time.Unix(0, env.GetTimestamp()).UTC().Format(RFC5424TimeOffsetNum)
	hostname = nilify(hostname)
	appID = nilify(appID)
	pid := nilify(generateProcessID(
		env.Tags["source_type"],
		env.InstanceId,
	))
	msg := appendNewline(removeNulls(env.GetLog().Payload))

	tmp := make([]byte, 0, 20+len(priority)+len(ts)+len(hostname)+len(appID)+len(pid)+len(msg))
	tmp = append(tmp, []byte("<"+priority+">1 ")...)
	tmp = append(tmp, []byte(ts+" ")...)
	tmp = append(tmp, []byte(hostname+" ")...)
	tmp = append(tmp, []byte(appID+" ")...)
	tmp = append(tmp, []byte(pid+" ")...)
	tmp = append(tmp, []byte("- - ")...)
	tmp = append(tmp, msg...)

	return tmp
}

func toRFC5424MetricMessage(env *loggregator_v2.Envelope, hostname, appID, structuredData string) []byte {
	ts := time.Unix(0, env.GetTimestamp()).UTC().Format(RFC5424TimeOffsetNum)
	hostname = nilify(hostname)
	appID = nilify(appID)
	pid := "[" + env.InstanceId + "]"
	priority := "14"

	message := make([]byte, 0, 20+len(priority)+len(ts)+len(hostname)+len(appID)+len(pid)+len(structuredData))
	message = append(message, []byte("<"+priority+">1 ")...)
	message = append(message, []byte(ts+" ")...)
	message = append(message, []byte(hostname+" ")...)
	message = append(message, []byte(appID+" ")...)
	message = append(message, []byte(pid+" - ")...)
	message = append(message, []byte(structuredData+" \n")...)

	return message
}

func genPriority(logType loggregator_v2.Log_Type) string {
	switch logType {
	case loggregator_v2.Log_OUT:
		return "14"
	case loggregator_v2.Log_ERR:
		return "11"
	default:
		return "-1"
	}
}

func nilify(x string) string {
	if x == "" {
		return "-"
	}
	return x
}