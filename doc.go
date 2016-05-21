/*
go-ipipnet-downloader提供针对ipip.net网站的数据下载器，支持数据文件本地或远程的更新监控和更新通知

Example:

    d := &Downloader{
        LocalPath: "data/mydata4vipweek2.dat",
        RemoteURL: "https://user.ipip.net/download.php?token=",
        CheckETag: true,
        ErrorCallback: func(err error) { fmt.Fprintf(os.Stderr, "%s", err.Error()) },
        UpdateCallback: func(path string) { fmt.Println("file updated:", path) },
    }
    d.EnsureLocal()
    go d.StartWatch()
*/

package downloader
