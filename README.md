# go-ipipnet-downloader

[![Build Status](https://travis-ci.org/yangchenxing/go-ipipnet-downloader.svg?branch=master)](https://travis-ci.org/yangchenxing/go-ipipnet-downloader)

go-ipipnet-downloader用于下载ipip.net提供的数据文件、监控和通知文件更新

## Installation
    go get -u github.com/yangchenxing/go-ipipnet-downloader

## Example

    d := &Downloader{
        LocalPath: "data/mydata4vipweek2.dat",
        RemoteURL: "https://user.ipip.net/download.php?token=",
        CheckETag: true,
        ErrorCallback: func(err error) { fmt.Fprintf(os.Stderr, "%s", err.Error()) },
        UpdateCallback: func(path string) { fmt.Println("file updated:", path) },
    }
    d.EnsureLocal()
    go d.StartWatch()
