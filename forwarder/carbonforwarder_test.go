package forwarder

import (
	"errors"
	"github.com/cep21/gohelpers/a"
	"github.com/cep21/gohelpers/workarounds"
	"github.com/golang/glog"
	"github.com/signalfuse/com_signalfuse_metrics_protobuf"
	"github.com/signalfuse/signalfxproxy/config"
	"github.com/signalfuse/signalfxproxy/core"
	"github.com/signalfuse/signalfxproxy/core/value"
	"github.com/signalfuse/signalfxproxy/listener"
	"testing"
	"time"
)

type mockConn struct {
	a.Conn
	deadlineReturn   error
	setDeadlineBlock chan bool
	writeReturn      error
}

func (conn *mockConn) SetDeadline(t time.Time) error {
	r := conn.deadlineReturn
	conn.deadlineReturn = nil
	conn.setDeadlineBlock <- true
	return r
}

func (conn *mockConn) Write(bytes []byte) (int, error) {
	r := conn.writeReturn
	conn.writeReturn = nil
	return len(bytes), r
}

func TestInvalidPort(t *testing.T) {

	ft := config.ForwardTo{
		Host: workarounds.GolangDoesnotAllowPointerToStringLiteral("invalid.port.address.should.not.bind"),
		Port: workarounds.GolangDoesnotAllowPointerToUint16Literal(1),
	}
	_, err := TcpGraphiteCarbonForwarerLoader(&ft)
	a.ExpectNotEquals(t, nil, err, "Expect an error")
}

type carbonDatapoint struct {
	core.Datapoint
	line string
}

func (dp *carbonDatapoint) ToCarbonLine() string {
	return dp.line
}

func TestCreation(t *testing.T) {
	listenFrom := config.ListenFrom{}
	// TODO: Enable :0 port and reading back the open port
	listenFrom.ListenAddr = workarounds.GolangDoesnotAllowPointerToStringLiteral("0.0.0.0:12345")
	forwardTo := newBasicBufferedForwarder(100, 1, "", 1)
	l, err := listener.CarbonListenerLoader(forwardTo, &listenFrom)
	defer l.Close()
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, 0, len(l.GetStats()), "Expect no stats")
	forwarder, err := newTcpGraphiteCarbonForwarer("0.0.0.0", 12345, time.Second, 10, "")
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, "", forwarder.Name(), "Expect no name")
	a.ExpectEquals(t, 0, len(forwarder.GetStats()), "Expect no stats")
	forwarder.openConnection = nil // Connection should remake itself
	timeToSend := time.Now().Round(time.Second)
	dpSent := core.NewAbsoluteTimeDatapoint("metric", map[string]string{}, value.NewIntWire(2), com_signalfuse_metrics_protobuf.MetricType_GAUGE, timeToSend)
	glog.Info("Sending a dp")
	forwarder.DatapointsChannel() <- dpSent
	glog.Info("Looking for DP back")
	dp := <-forwardTo.datapointsChannel
	a.ExpectEquals(t, "metric", dp.Metric(), "Expect metric back")
	a.ExpectEquals(t, dpSent.Timestamp(), dp.Timestamp(), "Expect metric back")
}

func TestDeadlineError(t *testing.T) {
	listenFrom := config.ListenFrom{}
	listenFrom.ListenAddr = workarounds.GolangDoesnotAllowPointerToStringLiteral("0.0.0.0:12246")

	forwardTo := newBasicBufferedForwarder(100, 1, "", 1)
	l, err := listener.CarbonListenerLoader(forwardTo, &listenFrom)
	defer l.Close()
	carbonForwarder, err := newTcpGraphiteCarbonForwarer("0.0.0.0", 12246, time.Second, 10, "")
	a.ExpectEquals(t, nil, err, "Expect no error")

	dpSent := core.NewRelativeTimeDatapoint("metric", map[string]string{}, value.NewIntWire(2), com_signalfuse_metrics_protobuf.MetricType_GAUGE, 0)
	mockConn := mockConn{
		setDeadlineBlock: make(chan bool),
	}
	mockConn.deadlineReturn = errors.New("deadline error")
	carbonForwarder.openConnection = &mockConn
	carbonForwarder.DatapointsChannel() <- dpSent
	a.ExpectEquals(t, 0, len(forwardTo.datapointsChannel), "Expect drain from chan")
	_ = <-mockConn.setDeadlineBlock
	a.ExpectEquals(t, 0, len(forwardTo.datapointsChannel), "Expect no stats")
}

func TestWriteError(t *testing.T) {
	listenFrom := config.ListenFrom{}
	listenFrom.ListenAddr = workarounds.GolangDoesnotAllowPointerToStringLiteral("0.0.0.0:12347")

	forwardTo := newBasicBufferedForwarder(100, 1, "", 1)
	l, err := listener.CarbonListenerLoader(forwardTo, &listenFrom)
	defer l.Close()
	forwarder, err := newTcpGraphiteCarbonForwarer("0.0.0.0", 12347, time.Second, 10, "")
	a.ExpectEquals(t, nil, err, "Expect no error")

	dpSent := core.NewRelativeTimeDatapoint("metric", map[string]string{}, value.NewIntWire(2), com_signalfuse_metrics_protobuf.MetricType_GAUGE, 0)
	mockConn := mockConn{
		setDeadlineBlock: make(chan bool),
	}
	mockConn.writeReturn = errors.New("write error")
	forwarder.openConnection = &mockConn
	forwarder.DatapointsChannel() <- dpSent
	a.ExpectEquals(t, 0, len(forwardTo.datapointsChannel), "Expect drain from chan")
	_ = <-mockConn.setDeadlineBlock
	a.ExpectEquals(t, 0, len(forwardTo.datapointsChannel), "Expect no stats")
}

func TestCarbonWrite(t *testing.T) {
	listenFrom := config.ListenFrom{}
	listenFrom.ListenAddr = workarounds.GolangDoesnotAllowPointerToStringLiteral("0.0.0.0:12348")
	forwardTo := newBasicBufferedForwarder(100, 1, "", 1)
	l, err := listener.CarbonListenerLoader(forwardTo, &listenFrom)
	defer l.Close()
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, 0, len(l.GetStats()), "Expect no stats")
	forwarder, err := newTcpGraphiteCarbonForwarer("0.0.0.0", 12348, time.Second, 10, "")
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, "", forwarder.Name(), "Expect no name")
	a.ExpectEquals(t, 0, len(forwarder.GetStats()), "Expect no stats")
	forwarder.openConnection = nil // Connection should remake itself
	dpSent := core.NewRelativeTimeDatapoint("metric", map[string]string{}, value.NewIntWire(2), com_signalfuse_metrics_protobuf.MetricType_GAUGE, 0)
	glog.Info("Sending a dp")
	carbonReadyDp := &carbonDatapoint{dpSent, "lineitem 3 4"}
	forwarder.DatapointsChannel() <- carbonReadyDp
	glog.Info("Looking for DP back")
	dp := <-forwardTo.datapointsChannel
	a.ExpectEquals(t, "lineitem", dp.Metric(), "Expect metric back")
	a.ExpectEquals(t, "3", dp.Value().WireValue(), "Expect value back")
}

func TestFailedConn(t *testing.T) {
	listenFrom := config.ListenFrom{}
	listenFrom.ListenAddr = workarounds.GolangDoesnotAllowPointerToStringLiteral("0.0.0.0:12349")
	forwardTo := newBasicBufferedForwarder(100, 1, "", 1)
	l, err := listener.CarbonListenerLoader(forwardTo, &listenFrom)
	defer l.Close()
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, 0, len(l.GetStats()), "Expect no stats")
	forwarder, err := newTcpGraphiteCarbonForwarer("0.0.0.0", 12349, time.Second, 10, "")
	a.ExpectEquals(t, nil, err, "Expect no error")
	a.ExpectEquals(t, "", forwarder.Name(), "Expect no name")
	a.ExpectEquals(t, 0, len(forwarder.GetStats()), "Expect no stats")
	forwarder.openConnection = nil // Connection should remake itself
	forwarder.connectionAddress = "0.0.0.0:1"
	dpSent := core.NewRelativeTimeDatapoint("metric", map[string]string{}, value.NewIntWire(2), com_signalfuse_metrics_protobuf.MetricType_GAUGE, 0)
	glog.Info("Sending a dp")
	forwarder.DatapointsChannel() <- dpSent
	glog.Info("Looking for DP back")
	a.ExpectEquals(t, 0, len(forwardTo.datapointsChannel), "Expect no stats")
}
