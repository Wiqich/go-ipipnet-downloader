package downloader

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var (
	errNotModified  = errors.New("not modified")
	defaultInterval = time.Hour
)

// Downloader 用于下载IPIPNet提供的数据文件、检查和通知文件更新
type Downloader struct {
	LocalPath      string
	RemoteURL      string
	Interval       time.Duration
	CheckETag      bool
	ErrorCallback  func(error)
	UpdateCallback func(string)
	etag           string
	watching       bool
}

// EnsureLocal 用于在首次加载前确保本地文件存在
func (d *Downloader) EnsureLocal() error {
	if _, err := os.Stat(d.LocalPath); err == nil {
		if d.CheckETag {
			etag, err := ioutil.ReadFile(d.LocalPath + ".etag")
			if err != nil {
				return fmt.Errorf("load etag fail: %s", err.Error())
			}
			d.etag = string(etag)
		}
		return nil
	}
	if err := d.download(); err != nil {
		return fmt.Errorf("download fail: %s", err.Error())
	}
	return nil
}

// StartWatch 开始监控数据文件变化
func (d *Downloader) StartWatch() {
	d.watching = true
	if d.RemoteURL == "" {
		go d.watchLocal()
	} else {
		go d.watchRemote()
	}
}

// StopWatch 停止监控数据文件变化
func (d *Downloader) StopWatch() {
	d.watching = false
}

func (d *Downloader) watchLocal() {
	interval := d.Interval
	if interval == 0 {
		interval = defaultInterval
	}
	info, _ := os.Stat(d.LocalPath)
	ts := info.ModTime()
	time.Sleep(interval)
	for d.watching {
		info, err := os.Stat(d.LocalPath)
		if err != nil {
			d.onError(err)
		} else if info.ModTime().After(ts) {
			fmt.Println("call update")
			d.onUpdate()
		}
		time.Sleep(interval)
	}
}

func (d *Downloader) watchRemote() {
	interval := d.Interval
	if interval == 0 {
		interval = defaultInterval
	}
	for d.watching {
		if err := d.download(); err == errNotModified {
			// do nothing
		} else if err != nil {
			d.onError(err)
		} else {
			d.onUpdate()
		}
		time.Sleep(interval)
	}
}

func (d *Downloader) onError(err error) {
	if d.ErrorCallback != nil {
		d.ErrorCallback(err)
	}
}

func (d *Downloader) onUpdate() {
	if d.UpdateCallback != nil {
		d.UpdateCallback(d.LocalPath)
	}

}

func (d *Downloader) download() error {
	if d.RemoteURL == "" {
		return errors.New("remote url is unset")
	}
	resp, err := http.Get(d.RemoteURL)
	if err != nil {
		return fmt.Errorf("download fail: %s", err.Error())
	}
	if !d.CheckETag {
		if err := saveStreamToFile(resp.Body, d.LocalPath); err != nil {
			return fmt.Errorf("save local file fail: %s", err.Error())
		}
		return nil
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		return fmt.Errorf("download fail: no etag")
	}
	if etag == d.etag {
		return errNotModified
	}
	if err := saveStreamToFile(resp.Body, d.LocalPath); err != nil {
		return fmt.Errorf("save local file fail: %s", err.Error())
	}
	if err := ioutil.WriteFile(d.LocalPath+".etag", []byte(etag), 0755); err != nil {
		return fmt.Errorf("save local etag file fail: %s", err.Error())
	}
	return nil
}

func saveStreamToFile(r io.Reader, path string) error {
	tempPath := path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("open temp file %q fail: %s", tempPath, err.Error())
	}
	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("copy stream to temp file %q fail: %s", tempPath, err.Error())
	}
	file.Close()
	os.Rename(tempPath, path)
	return nil
}
