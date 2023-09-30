# Video Uploading and Transcoding Pipeline - v1

We will be building a video uploading and transcoding service in Go. The service will accept video uploads, transcode them into multiple formats, and then them in our local file system.

## Features

- Upload video files to the server in chunks
- Save the original video file in the server
- Transcode the video file into multiple resolutions
- Save the transcoded video files in the server
- Get the list of all the video files in the server

## API Documentation

| Method | Endpoint   | Description                                                           | Request Body          | Response Body          |
| ------ | ---------- | --------------------------------------------------------------------- | --------------------- | ---------------------- |
| POST   | /upload    | Upload a video file to the server                                     | `UploadVideoRequest`  | `UploadVideoResponse`  |
| GET    | /file-info | Get the list of all video files both processed and unprocessed        | Nil                   | `GetFileInfoResponse`  |
| POST   | /process   | Process a video file ( start transcoding it to multiple resolutions ) | `ProcessVideoRequest` | `ProcessVideoResponse` |

### Data Models

1. #### UploadVideoRequest

```json
{
  "fileId": "1695838850878",
  "fileName": "sample.mkv",
  "fileChunk": "<file>"
}
```

2. #### UploadVideoResponse

```json
File chunk uploaded successfully
```

3. #### GetFileInfoResponse

```json
{
  "files": [
    {
      "1695791657548": {
        "file_name": "sample.mkv",
        "is_processed": true,
        "is_processing": true
      }
    }
  ]
}
```

4. #### ProcessVideoRequest

```json
{
  "fileId": "1695791657548"
}
```

5. #### ProcessVideoResponse

```json
Started processing
```

# Benchmarks

`File Size: 3.1GB`

`Time Started: 10:44:56 am`

`Time Ended: 14:51:08 pm`

`Time Taken: 4h 6min`

| Resolution | Time Taken | Size     |
| ---------- | ---------- | -------- |
| 144p       | 1h 4min    | 872.5 MB |
| 240p       | 1h 40min   | 1.0 GB   |
| 360p       | 2h 19min   | 1.3 GB   |
| 480p       | 2h 53min   | 1.7 GB   |
| 720p       | 3h 32min   | 2.5 GB   |
| 1080p      | 4h 6min    | 4.0 GB   |
