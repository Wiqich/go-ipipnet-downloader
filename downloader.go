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
	errNoETag       = errors.New("no etag")
	defaultInterval = time.Hour
)

// Downloader 用于下载IPIPNet提供的数据文件、检查和通知文件更新
type Downloader struct {
	// 必须的本地文件路径，对应ETag文件被保存在LocalPath+".etag"
	LocalPath string

	// 可选的远程下载地址，此字段为空时将监控本地文件修改，非空时监控远程文件ETag变化
	RemoteURL string

	// 监控周期
	Interval time.Duration

	// 是否检查远程下载中的ETag字段，若远程下载服务器出现异常导致缺少ETag字段，可将此字段设置为false以退化为周期性强制更新
	CheckETag bool

	// 错误事件回调函数，参数为错误对象
	ErrorCallback func(error)

	// 更新事件回调函数，参数为数据文件路径
	UpdateCallback func(string)

	etag     string
	watching bool
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

func (d *Downloader) checkRemoteModification() (bool, error) {
	resp, err := http.Head(d.RemoteURL)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		return true, errNoETag
	}
	return d.etag != etag, nil
}

func (d *Downloader) download() error {
	if d.RemoteURL == "" {
		return errors.New("remote url is unset")
	}
	// check remote modification first
	if d.CheckETag {
		if modified, err := d.checkRemoteModification(); err != nil {
			return fmt.Errorf("check remote modification fail:", err.Error())
		} else if !modified {
			return errNotModified
		}
	}
	// download remote content
	resp, err := http.Get(d.RemoteURL)
	if err != nil {
		return fmt.Errorf("download fail: %s", err.Error())
	}
	defer resp.Body.Close()
	if !d.CheckETag {
		if err := saveStreamToFile(resp.Body, d.LocalPath); err != nil {
			return fmt.Errorf("save local file fail: %s", err.Error())
		}
		return nil
	}
	// do not check etag while checkRemoteModification returns true
	// if etag == "" {
	// 	return fmt.Errorf("download fail: no etag")
	// }
	// if etag == d.etag {
	// 	return errNotModified
	// }
	if err := saveStreamToFile(resp.Body, d.LocalPath); err != nil {
		return fmt.Errorf("save local file fail: %s", err.Error())
	}
	if err := ioutil.WriteFile(d.LocalPath+".etag", []byte(resp.Header.Get("ETag")), 0755); err != nil {
		return fmt.Errorf("save local etag file fail: %s", err.Error())
	}
	d.etag = resp.Header.Get("ETag")
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
