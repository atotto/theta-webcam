package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"sync"

	"github.com/atotto/theta-webcam/stream"
)

var save = flag.String("save", "", "save motion jpeg (mov) file")

func main() {
	flag.Parse()

	endpoint := "http://192.168.1.1/osc/commands/execute"

	s, err := stream.NewLiveStream(endpoint)
	if err != nil {
		log.Fatal(err)
	}

	var f *os.File
	if *save != "" {
		f, err = os.Create(*save)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
	}

	var mu sync.RWMutex
	var jpegBuf = make([]byte, 0, 60000)

	log.Println("Start streaming")

	go func() {
		for {
			data, err := s.NextImage()
			if err != nil {
				log.Fatal(err)
			}
			mu.Lock()
			if len(jpegBuf) < len(data) {
				log.Printf("extend: %d", len(data))
				jpegBuf = append(jpegBuf[:0], data...)
			} else {
				copy(jpegBuf, data)
			}
			mu.Unlock()
			f.Write(data)
		}
	}()

	http.HandleFunc("/jpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		mu.RLock()
		_, err := w.Write(jpegBuf)
		mu.RUnlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/mjpeg", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serve streaming")
		m := multipart.NewWriter(w)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
		w.Header().Set("Connection", "close")
		header := textproto.MIMEHeader{}
		var buf bytes.Buffer
		for {
			mu.RLock()
			buf.Reset()
			_, err := buf.Write(jpegBuf)
			mu.RUnlock()
			if err != nil {
				log.Println(err)
				break
			}
			header.Set("Content-Type", "image/jpeg")
			header.Set("Content-Length", fmt.Sprint(buf.Len()))
			mw, err := m.CreatePart(header)
			if err != nil {
				break
			}
			mw.Write(buf.Bytes())
			if flusher, ok := mw.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		log.Println("Stop streaming")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/mjpeg" />`))
	})

	http.ListenAndServe(":8080", nil)
}
