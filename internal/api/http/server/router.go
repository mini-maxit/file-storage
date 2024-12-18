package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/mini-maxit/file-storage/utils"
	"github.com/sirupsen/logrus"
)

type Server struct {
	mux http.Handler
}

func (s *Server) Run(addr string) error {
	logrus.Infof("Server is running on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func NewServer(ts *services.TaskService) *Server {
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

		// Process the uploaded archive
		archiveFile, fileHeader, err := r.FormFile("archive")
		if err != nil {
			http.Error(w, "Archive file is required.", http.StatusBadRequest)
			return
		}
		defer utils.CloseIO(archiveFile)

		// Save the archive temporarily
		originalExt := filepath.Ext(fileHeader.Filename)
		tempArchivePath := filepath.Join(os.TempDir(), fmt.Sprintf("task_archive_%d%s", taskID, originalExt))
		tempArchive, err := os.Create(tempArchivePath)
		if err != nil {
			http.Error(w, "Failed to create temporary file for archive.", http.StatusInternalServerError)
			return
		}
		defer utils.RemoveDirectory(tempArchivePath)
		defer utils.CloseIO(tempArchive)

		if _, err := io.Copy(tempArchive, archiveFile); err != nil {
			http.Error(w, "Failed to save archive file.", http.StatusInternalServerError)
			return
		}

		// Decompress the archive to a temporary directory
		tempExtractPath := filepath.Join(os.TempDir(), fmt.Sprintf("task_%d", taskID))
		defer utils.RemoveDirectory(tempExtractPath)

		if err := utils.DecompressArchive(tempArchivePath, tempExtractPath); err != nil {
			http.Error(w, "Failed to decompress archive.", http.StatusInternalServerError)
			return
		}
		entries, err := os.ReadDir(tempExtractPath)
		if err != nil {
			log.Fatal(err)
		}
		if len(entries) != 1 {
			http.Error(w, "Task archive has to contain exactly 1 main folder", http.StatusBadRequest)
		}

		extractedPath := filepath.Join(tempExtractPath, entries[0].Name())

		// Prepare files map from decompressed folder structure
		filesMap := make(map[string][]byte)

		// Load description file
		descriptionPath := filepath.Join(extractedPath, "description.pdf")
		descriptionContent, err := os.ReadFile(descriptionPath)
		if err != nil {
			http.Error(w, "Description file is missing or unreadable in the archive.", http.StatusBadRequest)
			return
		}
		filesMap["src/description.pdf"] = descriptionContent

		// Load input files
		inputDir := filepath.Join(extractedPath, "input")
		inputFiles, err := os.ReadDir(inputDir)
		if err != nil {
			http.Error(w, "Input directory is missing in the archive.", http.StatusBadRequest)
			return
		}

		for _, file := range inputFiles {
			filePath := filepath.Join(inputDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, "Failed to read input file in the archive.", http.StatusInternalServerError)
				return
			}
			filesMap["src/input/"+file.Name()] = content
		}

		// Load output files
		outputDir := filepath.Join(extractedPath, "output")
		outputFiles, err := os.ReadDir(outputDir)
		if err != nil {
			http.Error(w, "Output directory is missing in the archive.", http.StatusBadRequest)
			return
		}

		for _, file := range outputFiles {
			filePath := filepath.Join(outputDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, "Failed to read output file in the archive.", http.StatusInternalServerError)
				return
			}
			filesMap["src/output/"+file.Name()] = content
		}

		// Invoke the service function
		serviceErr := ts.CreateTaskDirectory(taskID, filesMap, overwrite)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to create Task Directory", map[string]interface{}{
				"taskID":    taskID,
				"overwrite": overwrite,
			})
			return
		}

		_, err = w.Write([]byte("Task directory created successfully"))
		if err != nil {
			return
		}
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
		defer utils.CloseIO(file)

		// Read the file content
		fileContent, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read submission file.", http.StatusInternalServerError)
			return
		}

		// Invoke the service function to handle the submission
		submissionNumber, serviceErr := ts.CreateUserSubmission(taskID, userID, fileContent, fileHeader.Filename)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to create User Submission", map[string]interface{}{
				"taskID":   taskID,
				"userID":   userID,
				"fileName": fileHeader.Filename,
			})
			return
		}

		response := map[string]interface{}{
			"message":          "Submission created successfully",
			"submissionNumber": submissionNumber,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
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

		// Extract 'taskID', 'userID', and 'submissionNumber' from form data
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

		// Process the uploaded archive
		archiveFile, fileHeader, err := r.FormFile("archive")
		if err != nil {
			http.Error(w, "Archive file is required.", http.StatusBadRequest)
			return
		}
		defer utils.CloseIO(archiveFile)

		// Save the archive temporarily
		originalExt := filepath.Ext(fileHeader.Filename)
		tempArchivePath := filepath.Join(os.TempDir(), fmt.Sprintf("outputs_archive_%d%s", taskID, originalExt))
		tempArchive, err := os.Create(tempArchivePath)
		if err != nil {
			http.Error(w, "Failed to create temporary file for archive.", http.StatusInternalServerError)
			return
		}
		defer utils.RemoveDirectory(tempArchivePath)
		defer utils.CloseIO(tempArchive)

		if _, err := io.Copy(tempArchive, archiveFile); err != nil {
			http.Error(w, "Failed to save archive file.", http.StatusInternalServerError)
			return
		}

		// Decompress the archive to a temporary directory
		tempExtractPath := filepath.Join(os.TempDir(), fmt.Sprintf("task_outputs_%d", taskID))
		defer utils.RemoveDirectory(tempExtractPath)

		if err := utils.DecompressArchive(tempArchivePath, tempExtractPath); err != nil {
			http.Error(w, "Failed to decompress archive.", http.StatusInternalServerError)
			return
		}

		// Read files from the "user-output" folder in the decompressed archive
		outputsDir := filepath.Join(tempExtractPath, "user-output")
		outputFilesList, err := os.ReadDir(outputsDir)
		if err != nil {
			http.Error(w, "Outputs directory is missing in the archive.", http.StatusBadRequest)
			return
		}

		for _, file := range outputFilesList {
			filePath := filepath.Join(outputsDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, "Failed to read file in Outputs directory.", http.StatusInternalServerError)
				return
			}
			outputFiles[file.Name()] = content
		}

		// Store the output files in the service function
		serviceErr := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to store user outputs", map[string]interface{}{
				"taskID":     taskID,
				"userID":     userID,
				"submission": submissionNumberStr,
			})
			return
		}

		_, err = w.Write([]byte("Output files stored successfully"))
		if err != nil {
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
		tarFilePath, serviceErr := ts.GetTaskFiles(taskID)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to get task files", map[string]interface{}{
				"taskID": taskID,
			})
			return
		}
		defer utils.RemoveDirectory(tarFilePath)

		// Open the .tar.gz file
		tarFile, err := os.Open(tarFilePath)
		if err != nil {
			http.Error(w, "Failed to open task files archive.", http.StatusInternalServerError)
			return
		}
		defer utils.CloseIO(tarFile)

		// Set headers and serve the .tar.gz file
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=task%dFiles.tar.gz", taskID))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", utils.FileSize(tarFile)))

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
		fileContent, fileName, serviceErr := ts.GetUserSubmission(taskID, userID, submissionNumber)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to get user submission files", map[string]interface{}{
				"taskID":     taskID,
				"userID":     userID,
				"submission": submissionNumberStr,
			})
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
		tarFilePath, serviceErr := ts.GetInputOutput(taskID, inputOutputID)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to get input output files", map[string]interface{}{
				"taskID":        taskID,
				"inputOutputID": inputOutputID,
			})
			return
		}
		defer utils.RemoveDirectory(tarFilePath)

		// Open the .tar.gz file
		tarFile, err := os.Open(tarFilePath)
		if err != nil {
			http.Error(w, "Failed to open files archive.", http.StatusInternalServerError)
			return
		}
		defer utils.CloseIO(tarFile)

		// Set headers and serve the .tar.gz file
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=Task%dInputOutput%dFiles.tar.gz", taskID, inputOutputID))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", utils.FileSize(tarFile)))

		// Stream the file content to the response
		_, err = io.Copy(w, tarFile)
		if err != nil {
			http.Error(w, "Failed to send task files archive.", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/getSolutionPackage", func(w http.ResponseWriter, r *http.Request) {
		// Ensure the request method is GET
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract taskID, userID, and submissionNumber parameters from the URL query
		taskIDStr := r.URL.Query().Get("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}
		userIDStr := r.URL.Query().Get("userID")
		if userIDStr == "" {
			http.Error(w, "userID is required.", http.StatusBadRequest)
			return
		}
		submissionNumStr := r.URL.Query().Get("submissionNumber")
		if submissionNumStr == "" {
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
		submissionNum, err := strconv.Atoi(submissionNumStr)
		if err != nil {
			http.Error(w, "Invalid submissionNumber.", http.StatusBadRequest)
			return
		}

		// Call GetUserSolutionPackage to retrieve the package as a .tar.gz archive
		tarFilePath, serviceErr := ts.GetUserSolutionPackage(taskID, userID, submissionNum)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to get user submission files", map[string]interface{}{
				"taskID":        taskID,
				"userID":        userID,
				"submissionNum": submissionNum,
			})
			return
		}
		defer utils.RemoveDirectory(tarFilePath) // Clean up the temporary file after response

		// Open the .tar.gz file
		tarFile, err := os.Open(tarFilePath)
		if err != nil {
			http.Error(w, "Failed to open solution package.", http.StatusInternalServerError)
			return
		}
		defer utils.CloseIO(tarFile)

		// Set headers and serve the .tar.gz file
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=Task%d_User%d_Submission%d_Package.tar.gz", taskID, userID, submissionNum))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", utils.FileSize(tarFile)))

		// Stream the file content to the response
		_, err = io.Copy(w, tarFile)
		if err != nil {
			http.Error(w, "Failed to send solution package.", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/deleteTask", func(w http.ResponseWriter, r *http.Request) {
		// Ensure the request method is DELETE
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract taskID parameter from the URL query
		taskIDStr := r.URL.Query().Get("taskID")
		if taskIDStr == "" {
			http.Error(w, "taskID is required.", http.StatusBadRequest)
			return
		}

		// Convert taskID to an integer
		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		// Call DeleteTask to delete the specified task directory
		serviceErr := ts.DeleteTask(taskID)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to delete task", map[string]interface{}{
				"taskID": taskID,
			})
			return
		}

		// Respond with a success message
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(fmt.Sprintf("Task %d successfully deleted.", taskID)))
		if err != nil {
			http.Error(w, "Failed to send response.", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/getTaskDescription", func(w http.ResponseWriter, r *http.Request) {
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

		// Convert taskID to integer
		taskID, err := strconv.Atoi(taskIDStr)
		if err != nil {
			http.Error(w, "Invalid taskID.", http.StatusBadRequest)
			return
		}

		// Retrieve the task description file content
		fileContent, fileName, serviceErr := ts.GetTaskDescription(taskID)
		if serviceErr != nil {
			services.WriteServiceError(serviceErr, w, "Failed to get task description file", map[string]interface{}{
				"taskID": taskID,
			})
			return
		}

		// Set response headers to prompt file download with the original file name
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))

		// Write file content to the response
		if _, err := w.Write(fileContent); err != nil {
			http.Error(w, "Failed to write file content to response", http.StatusInternalServerError)
			return
		}
	})

	return &Server{mux: mux}
}
