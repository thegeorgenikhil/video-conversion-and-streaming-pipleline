package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"
)

const JSONPersistFileName = "./fileMap.json"

var fm FileInfoMap
var resMap map[string]string

// operation is a clean up function on shutting down
type operation func(ctx context.Context) error

type FileInfo struct {
	FileName       string `json:"file_name"`
	IsProcessed    bool   `json:"is_processed"`
	IsProcessingRN bool   `json:"is_processing"`
}

type FileInfoMap struct {
	fmap map[string]*FileInfo
	sync.Mutex
}

func (fm *FileInfoMap) WriteFileInfo(fileId string, fileName string) {
	fm.Lock()
	defer fm.Unlock()
	fm.fmap[fileId] = &FileInfo{
		FileName:       fileName,
		IsProcessed:    false,
		IsProcessingRN: false,
	}
}

func (fm *FileInfoMap) ReadFileInfo(fileId string) (*FileInfo, bool) {
	v, ok := fm.fmap[fileId]
	return v, ok
}

func (fm *FileInfoMap) ChangeVideoProcessedStatus(fileId string) {
	fm.Lock()
	defer fm.Unlock()
	fi := fm.fmap[fileId]

	fi.IsProcessed = true
}

func (fm *FileInfoMap) ChangeVideoIsProcessedStatusTrue(fileId string) {
	fm.Lock()
	defer fm.Unlock()
	fi := fm.fmap[fileId]

	fi.IsProcessingRN = true
}

func (fm *FileInfoMap) ChangeVideoIsProcessedStatusFalse(fileId string) {
	fm.Lock()
	defer fm.Unlock()
	fi := fm.fmap[fileId]

	fi.IsProcessingRN = false
}

func (fm *FileInfoMap) PersistMap() error {
	jsonData, err := json.Marshal(fm.fmap)

	if err != nil {
		log.Printf("Error occured while marshalling json %s", err)
		return err
	}

	jsonFile, err := os.Create(JSONPersistFileName)

	if err != nil {
		log.Printf("Error occured while creating json file %s", err)
		return err
	}

	jsonFile.Write(jsonData)

	log.Println("Data written to file")
	return nil
}

func main() {
	fm.fmap = make(map[string]*FileInfo)

	data, err := os.ReadFile(JSONPersistFileName)

	if err != nil {
		log.Println("Not able to read from the file")
		panic(err)
	}

	err = json.Unmarshal(data, &fm.fmap)

	if err != nil {
		log.Println("Not able to read from the file")
		panic(err)
	}

	resMap = map[string]string{
		// "4k":    "3840:2160",
		// "2k":    "2560:1440",
		// "1080p": "1920:1080",
		// "720p":  "1280:720",
		// "480p":  "854:480",
		// "360p":  "640:360",
		// "240p":  "426:240",
		"144p":  "256:144",
	}

	http.HandleFunc("/upload", uploadFile)
	http.HandleFunc("/file-info", getFileInfo)
	http.HandleFunc("/process-video", processVideo)
	http.Handle("/", http.FileServer(http.Dir("static")))

	go func() {
		if err := http.ListenAndServe(":8001", nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fm.PersistMap()
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	fileChunk, _, err := r.FormFile("fileChunk")
	fileId := r.FormValue("fileId")
	fileName := r.FormValue("fileName")

	if err != nil {
		http.Error(w, "Invalid file chunk", http.StatusBadRequest)
		return
	}
	defer fileChunk.Close()

	if _, ok := fm.ReadFileInfo(fileId); !ok {
		fm.WriteFileInfo(fileId, fileName)
	}

	// Create or append to the destination file
	dstFile, err := os.OpenFile(fmt.Sprintf("./upload/%s_%s", fileId, fileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		http.Error(w, "Error creating or opening destination file", http.StatusInternalServerError)
		return
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, fileChunk)
	if err != nil {
		http.Error(w, "Error copying file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File chunk uploaded successfully"))
}

func getFileInfo(w http.ResponseWriter, r *http.Request) {
	res, err := json.Marshal(fm.fmap)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Not able to get all the file info at the moment"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func processVideo(w http.ResponseWriter, r *http.Request) {
	fileId := r.FormValue("fileId")
	fm.ChangeVideoIsProcessedStatusTrue(fileId)

	go func(fileId string) {

		v, ok := fm.ReadFileInfo(fileId)

		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("File Id Not Present"))
			return
		}

		fileName := v.FileName
		filePath := fmt.Sprintf("./upload/%s_%s", fileId, fileName)

		log.Printf("STARTED: Processing for file %s_%s\n", fileId, fileName)

		var wg sync.WaitGroup

		for k, v := range resMap {
			wg.Add(1)
			go convertVideo(filePath, k, v, &wg)
		}

		wg.Wait()

		fm.ChangeVideoIsProcessedStatusFalse(fileId)
		fm.ChangeVideoProcessedStatus(fileId)
		log.Printf("FINISHED: Processing for file %s_%s\n", fileId, fileName)

	}(fileId)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Started processing"))
}

func convertVideo(filePath string, outputFilePrefix string, fileResolution string, wg *sync.WaitGroup) {
	defer wg.Done()
	splittedFilePath := strings.Split(filePath, "/")
	fileName := splittedFilePath[len(splittedFilePath)-1]

	cmd := fmt.Sprintf("ffmpeg -i '%s' -vf 'scale=%s' -acodec copy -c:a copy './static/videostore/%s_%s'", filePath, fileResolution, outputFilePrefix, fileName)
	log.Printf("STARTED: ffmpeg process for %s_%s\n", outputFilePrefix, fileName)
	cmdObj := exec.Command("bash", "-c", cmd)
	err := cmdObj.Run()
	if err != nil {
		log.Println("Failed to execute command: ", cmd)
	}
	log.Printf("FINISHED: ffmpeg process for %s_%s took \n", outputFilePrefix, fileName)
}

// gracefulShutdown waits for termination syscalls and doing clean up operations after received it
func gracefulShutdown(ctx context.Context, timeout time.Duration, ops map[string]operation) <-chan struct{} {
	wait := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)

		// add any other syscalls that you want to be notified with
		signal.Notify(s, os.Interrupt)
		<-s

		log.Println("shutting down")

		// set timeout for the ops to be done to prevent system hang
		timeoutFunc := time.AfterFunc(timeout, func() {
			log.Printf("timeout %d ms has been elapsed, force exit", timeout.Milliseconds())
			os.Exit(0)
		})

		defer timeoutFunc.Stop()

		var wg sync.WaitGroup

		// Do the operations asynchronously to save time
		for key, op := range ops {
			wg.Add(1)
			innerOp := op
			innerKey := key
			go func() {
				defer wg.Done()

				log.Printf("cleaning up: %s", innerKey)
				if err := innerOp(ctx); err != nil {
					log.Printf("%s: clean up failed: %s", innerKey, err.Error())
					return
				}

				log.Printf("%s was shutdown gracefully", innerKey)
			}()
		}

		wg.Wait()

		close(wait)
	}()

	return wait
}
