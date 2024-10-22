package server

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mini-maxit/file-storage/internal/api/http/initialization"
	"github.com/sirupsen/logrus"
)

type Server struct {
	mux http.Handler
}

func (s *Server) Run(addr string) error {
	logrus.Infof("Server is running on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func NewServer(init *initialization.Initialization) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World!"))
	},
	)

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10 MB
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "The uploaded file is too large.", http.StatusBadRequest)
			return
		}

		// Get the uploaded files
		files := r.MultipartForm.File["fileupload"] // "fileupload" is the form field name for the file input
		if len(files) == 0 {
			http.Error(w, "No files uploaded.", http.StatusBadRequest)
			return
		}

		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Create a file on the server to store the uploaded file
			f, err := os.OpenFile(fmt.Sprintf("./downloaded/%s", fileHeader.Filename), os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()

			// Copy the uploaded file data to the server file
			io.Copy(f, file)
		}
		w.Write([]byte("File uploaded successfully"))
	})

	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fileName := r.URL.Path[len("/download/"):]
		file, err := os.Open(fmt.Sprintf("./downloaded/%s", fileName))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Set the header for the file
		fileStat, err := file.Stat()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileStat.Size()))

		// Copy the file data to the response writer
		io.Copy(w, file)
	})

	return &Server{mux: mux}

}
