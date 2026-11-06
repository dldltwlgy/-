package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Album struct {
	Name   string   `json:"name"`
	Photos []string `json:"photos"`
}

type Data struct {
	Albums []Album `json:"albums"`
}

const (
	fileYing  = "albums.json"
	fileXiong = "albums_xiong.json"
	port      = "8080"
)

var mu sync.Mutex

// getalbums 传group
func dataFile(r *http.Request) string {
	if r.URL.Query().Get("group") == "xiong" {
		return fileXiong
	}
	return fileYing
}
// 上面返回文件， string是对应的文件名
func loadData(file string) (Data, error) {
	d := Data{Albums: []Album{}}//空切片
	raw, err := os.ReadFile(file)//读取文件json数据
	if err != nil {
		if os.IsNotExist(err) {//判断是不是文件不存在，如果是，就返回空相册
			return d, nil
		}
		
		return d, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {//trim是删除空格，string(raw)是将转换为字符串
		//如果文件内容为空，就返回空相册
		return d, nil
	}
	if err := json.Unmarshal(raw, &d); err != nil {//把json数据转换为d结构体·
		return d, err//现在的d已经有内容了
	}
	if d.Albums == nil {//判断相册是否为空返回空相册 
		d.Albums = []Album{}
	}
	
	
	return d, nil
}

func backdata(d Data, file string) error {
	raw, err := json.MarshalIndent(d, "", " ")//把d结构体转换为json数据，这里是为了写入磁盘
	if err != nil {
		return err
	}
	return os.WriteFile(file, raw, 0644)
}

func isImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {//拿到文件后缀，改为小写
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".mp4", ".webm":
		return true
	}
	return false
}

func main() {
	if err := os.MkdirAll("uploads", 0755); err != nil {//创建uploads文件夹
		fmt.Println("创建uploads文件夹失败:", err)
		panic(err)
	}
	http.HandleFunc("/api/albums", albumsAPI)//handle函数，处理相册请求
	http.HandleFunc("/upload", uploadAPI)
	http.HandleFunc("/", route)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}

func route(w http.ResponseWriter, r *http.Request) {//返回图片的
	path := r.URL.Path
	if strings.HasPrefix(path, "/uploads/") {
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))).ServeHTTP(w, r)
		return//删掉uploads/前缀，去uploads文件夹找文件
	}
	if path == "/" {//首页返回 lianxi.html
		http.ServeFile(w, r, "lianxi.html")
		return
	}
	if isImage(path) {//其他图片/视频直接从当前目录取
		http.ServeFile(w, r, "."+path)
		return
	}
	http.NotFound(w, r)
}

func albumsAPI(w http.ResponseWriter, r *http.Request) {//获取相册的 新建相册
	mu.Lock()//加锁，防止互串
	defer mu.Unlock()//解锁

	w.Header().Set("Content-Type", "application/json; charset=utf-8")//固定格式
	w.Header().Set("Cache-Control", "no-store")//不缓存
		file := dataFile(r)//data的return值
	d, err := loadData(file)//读取文件json数据
	if err != nil {
		http.Error(w, "失败了T~T", 500)
		return
	}

	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(d)//前端getalbums里await res.json()拿到的数据
		return
	}
	if r.Method == http.MethodPost {
		
			name := strconv.Itoa(len(d.Albums))//相册名称是相册数量把字符串转换为整数
		
		d.Albums = append(d.Albums, Album{Name: name, Photos: []string{}})//新建相册
		if err := backdata(d, file); err != nil {
			http.Error(w, "失败了T~T", 500)
			return
		}
		json.NewEncoder(w).Encode(d)
		return
	}
	http.Error(w, "方法不支持", http.StatusMethodNotAllowed)
}

func uploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	file := dataFile(r)//data的return值
	d, err := loadData(file)//读取文件json数据，返回相册
	if err != nil {
		http.Error(w, "失败了T~T", 500)
		return
	}

	index, err := strconv.Atoi(r.URL.Query().Get("album"))
	if err != nil {
		http.Error(w, "失败了T~T", 500)
		return
	}
	if index < 0 || index >= len(d.Albums) {//index上面规定过的相册名
		http.Error(w, "相册不存在失败了T~T", 500)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "失败了T~T", 500)
		return
	}
		
	files := r.MultipartForm.File["photos"]//对应前端叫photos的文件
	for _, fh := range files {
		src, err := fh.Open()//打开文件
			if err != nil {
			continue
		}
		name := filepath.Base(fh.Filename)//固定函数 取文件名
		dstPath := filepath.Join("uploads", name)//固定函数 拼接路径保存到uploads文件夹

		dst, err := os.Create(dstPath)//创建新图片文件
		if err != nil {
			src.Close()
			continue
		}
		_, copyErr := io.Copy(dst, src)//从前端读到的 文件内容 写入本地新图片文件
		dst.Close()
		src.Close()
		if copyErr != nil {
			continue
		}
		d.Albums[index].Photos = append(d.Albums[index].Photos, "/uploads/"+name)//加访问路径
	}

	if err := backdata(d, file); err != nil {
		http.Error(w, "失败了T~T", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(d)
}
