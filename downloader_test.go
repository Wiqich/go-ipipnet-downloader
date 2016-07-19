package downloader

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

type testServer struct {
	content  []byte
	etag     string
	useEtag  bool
	notFound bool
}

func (s *testServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if s.notFound {
		resp.WriteHeader(http.StatusNotFound)
		return
	}
	if s.useEtag {
		resp.Header().Set("ETag", s.etag)
	}
	resp.WriteHeader(http.StatusOK)
	resp.Write(s.content)
}

func (s *testServer) hide() {
	s.notFound = true
}

func (s *testServer) show() {
	s.notFound = false
}

var (
	server = testServer{
		content: []byte("test content"),
		etag:    "TEST_ETAG",
		useEtag: true,
	}
)

func TestMain(m *testing.M) {
	go http.ListenAndServe("127.0.0.1:8787", &server)
	os.Exit(m.Run())
}

func TestEnsureLocalCase1(t *testing.T) {
	// 测试本地文件存在的情况
	d := &Downloader{
		LocalPath: "test.txt",
		CheckETag: true,
	}
	if err := ioutil.WriteFile("test.txt", []byte("test"), 0755); err != nil {
		t.Error("write temp.txt fail:", err.Error())
		t.FailNow()
	}
	if err := ioutil.WriteFile("test.txt.etag", []byte("test"), 0755); err != nil {
		t.Error("write temp.txt.etag fail:", err.Error())
		t.FailNow()
	}
	defer os.Remove("test.txt")
	defer os.Remove("test.txt.etag")
	if err := d.EnsureLocal(); err != nil {
		t.Error("EnsureLocal fail:", err.Error())
		t.FailNow()
	}
	os.Remove("test.txt.etag")
	if err := d.EnsureLocal(); err == nil {
		t.Error("EnsureLocal pass unexpected: do not check etag file")
		t.FailNow()
	}
}

func TestEnsureLocalCase2(t *testing.T) {
	// 测试本地文件不存在的情况
	os.Remove("test.txt")
	defer os.Remove("test.txt")
	defer os.Remove("test.txt.etag")
	expectedContent := []byte("test content")
	server.content = expectedContent
	d := &Downloader{
		LocalPath: "test.txt",
		RemoteURL: "http://127.0.0.1:8787",
		CheckETag: true,
	}
	if err := d.EnsureLocal(); err != nil {
		t.Error("EnsureLocal fail:", err.Error())
		t.FailNow()
	}
	content, err := ioutil.ReadFile("test.txt")
	if err != nil {
		t.Error("read test.txt fail:", err.Error())
		t.FailNow()
	}
	if !bytes.Equal(content, expectedContent) {
		t.Errorf("unexpected content: expected=%v, actual=%v\n", expectedContent, content)
		t.FailNow()
	}
	os.Remove("test.txt")
	os.Remove("test.txt.etag")
	server.hide()
	defer server.show()
	if err := d.EnsureLocal(); err == nil {
		t.Error("EnsureLocal pass unexpected: download should be failed")
		t.FailNow()
	}
}

func TestWatchLocal(t *testing.T) {
	updateEvent := make(chan string)
	var errorEvent error
	d := &Downloader{
		LocalPath: "test.txt",
		Interval:  time.Microsecond * 500,
		UpdateCallback: func(path string) {
			updateEvent <- path
		},
		ErrorCallback: func(err error) {
			errorEvent = err
		},
	}
	if err := ioutil.WriteFile("test.txt", []byte("test"), 0755); err != nil {
		t.Error("write temp.txt fail:", err.Error())
		t.FailNow()
	}
	defer os.Remove("test.txt")
	go d.StartWatch()
	defer d.StopWatch()
	time.Sleep(time.Second)
	if err := ioutil.WriteFile("test.txt", []byte("testtest"), 0755); err != nil {
		t.Error("rewrite temp.txt fail:", err.Error())
		t.FailNow()
	}
	select {
	case <-time.After(time.Second * 3):
		t.Error("wait update event timeout")
		t.FailNow()
	case <-updateEvent:
	}
	os.Remove("test.txt")
	time.Sleep(time.Second)
	if errorEvent == nil {
		t.Error("error should be raised while text.txt removed")
		t.FailNow()
	}
}

func TestNoWatch(t *testing.T) {
	d := &Downloader{
		Interval: 0,
	}
	d.StartWatch()
	time.Sleep(time.Microsecond * 100)
	if d.watching {
		t.Error("unexpected watching status")
		return
	}
}
