package services

import (
	"encoding/json"
	"log"
	"net/http"
)

// ServiceError is an interface representing a common error type for service errors.
type ServiceError interface {
	Error() string
	StatusCode() int
}

// BadRequestError indicates an error caused by an invalid client request.
type BadRequestError struct {
	Message string
}

func (e *BadRequestError) Error() string {
	return e.Message
}

func (e *BadRequestError) StatusCode() int {
	return http.StatusBadRequest
}

// InternalServerError indicates an error that occurred on the server side.
type InternalServerError struct {
	Message string
}

func (e *InternalServerError) Error() string {
	return e.Message
}

func (e *InternalServerError) StatusCode() int {
	return http.StatusInternalServerError
}

func NewBadRequestError(message string) *BadRequestError {
	return &BadRequestError{Message: message}
}

func NewInternalServerError(message string) *InternalServerError {
	return &InternalServerError{Message: message}
}

// WriteServiceError handles service errors and writes an HTTP error response in JSON format,
// including additional context if provided.
func WriteServiceError(err ServiceError, w http.ResponseWriter, message string, context map[string]interface{}) {
	// Build the response payload
	response := map[string]interface{}{
		"reason":  message,
		"details": err.Error(),
	}

	// Include context if provided
	if len(context) > 0 {
		response["context"] = context
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode())

	// Encode the response to JSON and write it to the response writer
	jsonResponse, _ := json.Marshal(response)
	_, writeError := w.Write(jsonResponse)
	if writeError != nil {
		log.Println(writeError)
		return
	}
}

// BadRequestErrors
var (
	ErrDirectoryAlreadyExists      = NewBadRequestError("the task directory already exists, overwrite not allowed")
	ErrInvalidTaskID               = NewBadRequestError("invalid taskID: task directory does not exist")
	ErrFileHasNoExtension          = NewBadRequestError("file has no extension")
	ErrFileExtensionNotAllowed     = NewBadRequestError("file extension is not allowed")
	ErrOutputDirContainsFiles      = NewBadRequestError("output directory already contains files")
	ErrNoProgramFileFound          = NewBadRequestError("no program file found in submission")
	ErrMultipleProgramFilesFound   = NewBadRequestError("multiple program files found in submission")
	ErrInputFileDoesNotExist       = NewBadRequestError("input file does not exist")
	ErrOutputFileDoesNotExist      = NewBadRequestError("output file does not exist")
	ErrOutputFileMismatch          = NewBadRequestError("number of output files does not match the expected number")
	ErrUnexpectedOutputFileNumber  = NewBadRequestError("unexpected output file number provided")
	ErrDuplicateOutputFileNumber   = NewBadRequestError("duplicate output file number found in user submission")
	ErrInvalidOutputFileFormat     = NewBadRequestError("output file does not match the required format {number}.out")
	ErrSubmissionDirDoesNotExist   = NewBadRequestError("submission directory does not exist")
	ErrFailedReadOutputDirectory   = NewBadRequestError("failed reading output directory")
	ErrInvalidOutputFileNumber     = NewBadRequestError("invalid output file number provided")
	ErrTaskSrcDirDoesNotExist      = NewBadRequestError("task src directory does not exist")
	ErrInputDirectoryDoesNotExist  = NewBadRequestError("input src directory does not exist")
	ErrOutputDirectoryDoesNotExist = NewBadRequestError("output src directory does not exist")
	ErrFailedSearchSolutionFile    = NewBadRequestError("failed searching solution file")
	ErrSolutionFileDoesNotExist    = NewBadRequestError("solution file does not exist")
	ErrDescriptionFileDoesNotExist = NewBadRequestError("description file does not exist")
)

// InternalServerErrors
var (
	ErrFailedBackupDirectory         = NewInternalServerError("failed to backup existing directory")
	ErrFailedRemoveDirectory         = NewInternalServerError("failed to remove existing directory")
	ErrFailedRestoreDirectory        = NewInternalServerError("failed to restore existing directory")
	ErrFailedCreateDirectory         = NewInternalServerError("failed to create directory structure")
	ErrFailedValidateFiles           = NewInternalServerError("failed to validate files")
	ErrFailedCreateDescription       = NewInternalServerError("failed to create description.pdf")
	ErrFailedSaveUserFile            = NewInternalServerError("failed to save user file")
	ErrFailedCreateSubmissionDir     = NewInternalServerError("failed to create submission or output directory")
	ErrFailedGetSubmissionNumber     = NewInternalServerError("failed to get next submission number")
	ErrFailedSaveOutputFile          = NewInternalServerError("failed to save output file")
	ErrFailedGetInputOutputFile      = NewInternalServerError("failed to fetch input/output files")
	ErrFailedCreateTarFile           = NewInternalServerError("failed to create tar file")
	ErrFailedDeleteTaskDirectory     = NewInternalServerError("failed to delete task directory")
	ErrFailedSaveFiles               = NewInternalServerError("failed to save input output files")
	ErrFailedAccessOutputDirectory   = NewInternalServerError("failed to access output directory")
	ErrFailedAccessFile              = NewInternalServerError("failed to access file")
	ErrFailedDetermineRelPath        = NewInternalServerError("failed to determine relative path")
	ErrFailedCreateTarHeader         = NewInternalServerError("failed to create tar header")
	ErrFailedWriteTarHeader          = NewInternalServerError("failed to write tar header")
	ErrFailedOpenFile                = NewInternalServerError("failed to open file")
	ErrFailedWriteFileToTar          = NewInternalServerError("failed to write file to tar")
	ErrFailedAddFilesToTar           = NewInternalServerError("failed to add files to tar")
	ErrFailedReadSubmissionDirectory = NewInternalServerError("failed to read submission directory")
	ErrFailedReadProgramFile         = NewInternalServerError("failed to read program file")
	ErrFailedGetFileInfo             = NewInternalServerError("failed to get file info")
	ErrFailedAccessTaskDirectory     = NewInternalServerError("failed to access task directory")
	ErrMultipleSolutionFilesFound    = NewInternalServerError("multiple solution files found in submission")
	ErrFailedReadInputFiles          = NewInternalServerError("failed to read input/output files")
	ErrFailedReadOutputFiles         = NewInternalServerError("failed to read output file")
	ErrFailedToSaveCompileError      = NewInternalServerError("failed to save compile error")
	ErrFailedReadDescriptionFile     = NewInternalServerError("failed to read description.pdf")
)
