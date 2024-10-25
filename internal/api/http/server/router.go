package server

import (
	"fmt"
	"github.com/mini-maxit/file-storage/internal/api/http/initialization"
	"github.com/mini-maxit/file-storage/internal/helpers"
	"github.com/mini-maxit/file-storage/internal/services"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strconv"
)

type Server struct {
	mux http.Handler
}

func (s *Server) Run(addr string) error {
	logrus.Infof("Server is running on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func NewServer(init *initialization.Initialization, ts *services.TaskService) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/createTask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit the size of the incoming request
		r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024) // 50 MB limit

		// Parse the multipart form data
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			http.Error(w, "The uploaded files are too large.", http.StatusBadRequest)
			return
		}

		// Extract 'taskID' from form data
		taskIDStr := r.FormValue("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}
		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		// Extract 'overwrite' flag from form data
		overwriteStr := r.FormValue("overwrite")
		overwrite := false
		if overwriteStr != "" {
			overwrite, err = strconv.ParseBool(overwriteStr)
			if err != nil {
				http.Error(w, "Invalid overwrite flag.", http.StatusBadRequest)
				return
			}
		}

		// Prepare the files map
		filesMap := make(map[string][]byte)

		// Process the description file
		descriptionFile, _, err := r.FormFile("description")
		if err != nil {
			http.Error(w, "Description file is required.", http.StatusBadRequest)
			return
		}
		defer descriptionFile.Close()
		descriptionContent, err := io.ReadAll(descriptionFile)
		if err != nil {
			http.Error(w, "Failed to read description file.", http.StatusInternalServerError)
			return
		}
		filesMap["src/description.pdf"] = descriptionContent

		// Process input files
		inputFiles := r.MultipartForm.File["inputFiles"]
		for _, fileHeader := range inputFiles {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open input file.", http.StatusInternalServerError)
				return
			}
			defer file.Close()
			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read input file.", http.StatusInternalServerError)
				return
			}
			filesMap["src/input/"+fileHeader.Filename] = content
		}

		// Process output files
		outputFiles := r.MultipartForm.File["outputFiles"]
		for _, fileHeader := range outputFiles {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open output file.", http.StatusInternalServerError)
				return
			}
			defer file.Close()
			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read output file.", http.StatusInternalServerError)
				return
			}
			filesMap["src/output/"+fileHeader.Filename] = content
		}

		// Invoke the service function
		err = ts.CreateTaskDirectory(taskID, filesMap, overwrite)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create task directory: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Task directory created successfully"))
	})

	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit the size of the incoming request to 10 MB
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

		// Parse the multipart form data
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "The uploaded file is too large.", http.StatusBadRequest)
			return
		}

		// Extract 'taskID' and 'userID' from form data
		taskIDStr := r.FormValue("taskID")
		userIDStr := r.FormValue("userID")
		if taskIDStr == "" || userIDStr == "" {
			http.Error(w, "taskID and userID are required.", http.StatusBadRequest)
			return
		}

		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			http.Error(w, "Invalid userID.", http.StatusBadRequest)
			return
		}

		// Process the submission file
		file, fileHeader, err := r.FormFile("submissionFile")
		if err != nil {
			http.Error(w, "Submission file is required.", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Read the file content
		fileContent, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read submission file.", http.StatusInternalServerError)
			return
		}

		// Invoke the service function to handle the submission
		err = ts.CreateUserSubmission(taskID, userID, fileContent, fileHeader.Filename)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create submission: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Submission created successfully"))
	})

	mux.HandleFunc("/storeOutputs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit the size of the incoming request to 10 MB
		r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

		// Parse the multipart form data
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "The uploaded files are too large.", http.StatusBadRequest)
			return
		}

		// Extract 'taskID' and 'userID' from form data
		taskIDStr := r.FormValue("taskID")
		userIDStr := r.FormValue("userID")
		submissionNumberStr := r.FormValue("submissionNumber")
		if taskIDStr == "" || userIDStr == "" {
			http.Error(w, "taskID and userID are required.", http.StatusBadRequest)
			return
		}

		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			http.Error(w, "Invalid userID.", http.StatusBadRequest)
			return
		}

		submissionNumber, err := strconv.Atoi(submissionNumberStr)
		if err != nil {
			http.Error(w, "Invalid submission number.", http.StatusBadRequest)
			return
		}

		// Prepare maps for output files and error file
		outputFiles := make(map[string][]byte)
		errorFile := make(map[string][]byte)

		// Process the output files
		outputFilesUploaded := r.MultipartForm.File["outputs"]
		for _, fileHeader := range outputFilesUploaded {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open output file.", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read output file.", http.StatusInternalServerError)
				return
			}

			outputFiles[fileHeader.Filename] = content
		}

		// Process the error file
		errorFilesUploaded := r.MultipartForm.File["error"]
		for _, fileHeader := range errorFilesUploaded {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Failed to open error file.", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read error file.", http.StatusInternalServerError)
				return
			}

			errorFile[fileHeader.Filename] = content
		}

		// Check the conditions:
		if len(outputFiles) == 0 && len(errorFile) == 0 {
			http.Error(w, "Either outputs or error file must be provided.", http.StatusBadRequest)
			return
		}

		if len(outputFiles) > 0 && len(errorFile) > 0 {
			http.Error(w, "Cannot have both outputs and error file at the same time.", http.StatusBadRequest)
			return
		}

		// If output files are provided, store them
		if len(outputFiles) > 0 {
			err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to store output files: %v", err), http.StatusInternalServerError)
				return
			}
			w.Write([]byte("Output files stored successfully"))
			return
		}

		// If an error file is provided, store it
		if len(errorFile) > 0 {
			err := ts.StoreUserOutputs(taskID, userID, submissionNumber, errorFile)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to store error file: %v", err), http.StatusInternalServerError)
				return
			}
			w.Write([]byte("Error file stored successfully"))
			return
		}
	})

	mux.HandleFunc("/getTaskFiles", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract 'taskID' from query parameters
		taskIDStr := r.URL.Query().Get("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}

		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		// Call GetTaskFiles to retrieve the task files as a .tar.gz archive
		tarFilePath, err := ts.GetTaskFiles(taskID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to retrieve task files: %v", err), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tarFilePath)

		// Open the .tar.gz file
		tarFile, err := os.Open(tarFilePath)
		if err != nil {
			http.Error(w, "Failed to open task files archive.", http.StatusInternalServerError)
			return
		}
		defer tarFile.Close()

		// Set headers and serve the .tar.gz file
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=task%dFiles.tar.gz", taskID))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", helpers.FileSize(tarFile)))

		// Stream the file content to the response
		_, err = io.Copy(w, tarFile)
		if err != nil {
			http.Error(w, "Failed to send task files archive.", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/getUserSubmission", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract 'taskID' from query parameters
		taskIDStr := r.URL.Query().Get("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}

		// Extract 'userID' from query parameters
		userIDStr := r.URL.Query().Get("userID")
		if userIDStr == "" {
			http.Error(w, "userID is required.", http.StatusBadRequest)
			return
		}

		// Extract 'submissionNumber' from query parameters
		submissionNumberStr := r.URL.Query().Get("submissionNumber")
		if submissionNumberStr == "" {
			http.Error(w, "submissionNumber is required.", http.StatusBadRequest)
			return
		}

		// Convert parameters to integers
		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			http.Error(w, "Invalid userID.", http.StatusBadRequest)
			return
		}

		submissionNumber, err := strconv.Atoi(submissionNumberStr)
		if err != nil {
			http.Error(w, "Invalid submission number.", http.StatusBadRequest)
			return
		}

		// Retrieve the user's submission file content
		fileContent, fileName, err := ts.GetUserSubmission(taskID, userID, submissionNumber)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to retrieve submission file: %v", err), http.StatusInternalServerError)
			return
		}

		// Set response headers to prompt file download with the original file name
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))

		// Write file content to the response
		if _, err := w.Write(fileContent); err != nil {
			http.Error(w, "Failed to write file content to response", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/getInputOutput", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract parameters
		taskIDStr := r.URL.Query().Get("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}

		inputOutputIDStr := r.URL.Query().Get("inputOutputID")
		if inputOutputIDStr == "" {
			http.Error(w, "inputOutputID is required.", http.StatusBadRequest)
			return
		}

		// Convert parameters to integers
		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		inputOutputID, err := strconv.Atoi(inputOutputIDStr)
		if err != nil {
			http.Error(w, "Invalid inputOutputID.", http.StatusBadRequest)
			return
		}

		// Call GetTaskFiles to retrieve the task files as a .tar.gz archive
		tarFilePath, err := ts.GetInputOutput(taskID, inputOutputID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to retrieve Input and Output files: %v", err), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tarFilePath)

		// Open the .tar.gz file
		tarFile, err := os.Open(tarFilePath)
		if err != nil {
			http.Error(w, "Failed to open files archive.", http.StatusInternalServerError)
			return
		}
		defer tarFile.Close()

		// Set headers and serve the .tar.gz file
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=Task%dInputOutput%dFiles.tar.gz", taskID, inputOutputID))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", helpers.FileSize(tarFile)))

		// Stream the file content to the response
		_, err = io.Copy(w, tarFile)
		if err != nil {
			http.Error(w, "Failed to send task files archive.", http.StatusInternalServerError)
			return
		}
	})

	return &Server{mux: mux}
}
