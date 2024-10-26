# File Storage API

This API enables users to manage tasks, submit files, and store outputs generated by users' programs. Below is the documentation for each available route.

## Endpoints
### 1. Create Task
- Endpoint: /createTask
- Method: POST
- Description: Creates a new task directory with a specified structure, including input and output files.

#### Request Body (Form-Data):

- taskID (required): Integer value representing the unique task identifier.
- overwrite (optional): Boolean value indicating whether to overwrite an existing task directory.
- description (required): File upload for the task description (e.g., description.pdf). 
- inputFiles (required): Array of file uploads representing the input files.
- outputFiles (required): Array of file uploads representing the output files.

#### Request example: 

```bash
  curl -X POST http://localhost:8080/createTask \
  -F "taskID=1" \
  -F "overwrite=false" \
  -F "description=@/path/to/description.pdf" \
  -F "inputFiles=@/path/to/input1.txt" \
  -F "inputFiles=@/path/to/input2.txt" \
  -F "outputFiles=@/path/to/output1.txt" \
  -F "outputFiles=@/path/to/output2.txt"
```

#### Response:

- Success: 200 OK with the message "Task directory created successfully"
- Failure: 400 or 500 error code with a specific error message.

## 2. Submit File

- Endpoint: /submit
- Method: POST
- Description: Submits a file for a specific user and task.

#### Request Body (Form-Data):

- taskID (required): Integer ID of the task.
- userID (required): Integer ID of the user submitting the file.
- submissionFile (required): The file the user wants to submit (e.g., solution.c or similar).

#### Request example: 

```bash
  curl -X POST http://localhost:8080/submit \
  -F "taskID=1" \
  -F "userID=1" \
  -F "submissionFile=@/path/to/solution.c"
```

#### Response:

- Success: 200 OK with the message "Submission created successfully"
- Failure: 400 or 500 error code with a specific error message.

### 3. Store Outputs

- Endpoint: /storeOutputs
- Method: POST
- Description: Stores output files generated by the user's program under a specific submission directory. This can either be a set of output files or a compile error file.

#### Request Body (Form-Data):

- taskID (required): Integer ID of the task.
- userID (required): Integer ID of the user.
- submissionNumber (required): Integer indicating the submission version for which the output files are stored.
- outputs (optional): Array of output file uploads. Only .txt files are allowed.
- error (optional): File upload of a single error file named compile-error.err.

#### Constraints:

- Either outputs or error must be provided, but not both.
- The number of files in outputs must match the expected number specified in the taskID/src/output folder.

#### Request example (with output files):

```bash
  curl -X POST http://localhost:8080/storeOutputs \
  -F "taskID=1" \
  -F "userID=1" \
  -F "submissionNumber=1" \
  -F "outputs=@/path/to/output1.txt" \
  -F "outputs=@/path/to/output2.txt"
```

#### Request example (with an error file):

```bash
  curl -X POST http://localhost:8080/storeOutputs \
  -F "taskID=1" \
  -F "userID=1" \
  -F "submissionNumber=1" \
  -F "error=@/path/to/compile-error.err"
```

#### Response:

- Success:
    - 200 OK with "Output files stored successfully" if outputs were provided.
    - 200 OK with "Error file stored successfully" if error was provided.
- Failure: 400 or 500 error code with a specific error message.

### 4. Get Task Files
- Endpoint: /getTaskFiles
- Method: GET
- Description: Retrieves all files (description, input, and output) for a given task.

#### Query Params:
- taskID (required): Integer ID of the task.

Request example:

```bash
  curl --location 'http://localhost:8080/getTaskFiles?taskID=123'
```

#### Response:
- Success: Returns a .tar.gz file containing the task's src folder, named as task{taskID}Files.tar.gz. The archive includes:
  - description.pdf file if present
  - input/ folder with all input .txt files
  - output/ folder with all output .txt files 
- Failure: 400 or 500 error code with a specific error message.

### 5. Get User Submission
- Endpoint: /getUserSubmission
- Method: GET
- Description: Fetches the specific submission file for a user. It will replace the current getFiles from solution.go. For now it should check if there is only one program file.

#### Query Params:
- taskID (required): Integer ID of the task.
- userID (required): Integer ID of the user.
- submissionNumber (required): Integer indicating the submission version for which the output files are stored.

#### Request example:

```bash
  curl --location 'http://localhost:8080/getUserSubmission?taskID=123&userID=1&submissionNumber=1'
```

#### Response:
- Success: Returns a file containing user's solution for the requested submission
- Failure: 400 or 500 error code with a specific error message.

### 5. Get Input/Output Files
    Endpoint: /getInputOutput
    Method: GET
    Description: Retrieves specific input and output files for a given task.

#### Query Params:
- taskID (required): Integer ID of the task.
- inputOutputID (required): Integer ID of the input/output of the task.

#### Request example:

```bash
  curl --location 'http://localhost:8080/getInputOutput?taskID=123&inputOutputID=1'
```

#### Response:
- Success: Returns a .tar.gz file containing the task's src folder, named as Task{taskID}InputOutput{inputOutputID}Files.tar.gz. The archive includes:
  - input and output files
- Failure: 400 or 500 error code with a specific error message.

### 6. Delete Task

- Endpoint: /deleteTask
- Method: DELETE
- Description: Deletes the entire directory and all files associated with a specific task.

#### Query Params:
- taskID (required): Integer ID of the task to be deleted.

#### Request example:

```bash
  curl --location --request DELETE 'http://localhost:8080/deleteTask?taskID=123'
```

#### Response:
- Success: 200 OK with the message "Task {taskID} successfully deleted."
- Failure:
  - 400 Bad Request if taskID is missing or invalid.
  - 404 Not Found if the task directory does not exist.
  - 500 Internal Server Error if there is an error during the deletion process.