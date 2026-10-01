package qqbotsdk

import (
	"net/http"
	"testing"
)

// TestUploadC2CFileByURL reproduces the documented URL upload.
func TestUploadC2CFileByURL(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"file_uuid":"uuid_a1b2c3d4e5f6","file_info":"AE86C5D3","ttl":300}`)

	resp, err := client.UploadC2CFile(t.Context(), "USER1", &FileUploadRequest{
		FileType:   FileTypeImage,
		URL:        "https://example.com/image.png",
		SrvSendMsg: false,
	})
	if err != nil {
		t.Fatalf("UploadC2CFile: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/v2/users/USER1/files" {
		t.Errorf("path = %s, want the documented address", req.Path)
	}

	body := decodeBody(t, req)
	if body["file_type"] != float64(FileTypeImage) {
		t.Errorf("file_type = %v, want %d", body["file_type"], FileTypeImage)
	}
	if body["url"] != "https://example.com/image.png" {
		t.Errorf("url = %v", body["url"])
	}
	// srv_send_msg=false is the zero value, so it is omitted.
	if _, present := body["srv_send_msg"]; present {
		t.Errorf("srv_send_msg = %v, want it omitted", body["srv_send_msg"])
	}

	if resp.FileInfo != "AE86C5D3" || resp.FileUUID != "uuid_a1b2c3d4e5f6" || resp.TTL != 300 {
		t.Errorf("resp = %+v, want the documented fields", resp)
	}
}

// TestUploadGroupFile checks the group upload endpoint.
func TestUploadGroupFile(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"file_uuid":"u","file_info":"FI","ttl":0}`)

	if _, err := client.UploadGroupFile(t.Context(), "GROUP1", &FileUploadRequest{
		FileType:   FileTypeVideo,
		URL:        "https://example.com/v.mp4",
		SrvSendMsg: true,
	}); err != nil {
		t.Fatalf("UploadGroupFile: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/v2/groups/GROUP1/files" {
		t.Errorf("path = %s, want the group upload address", req.Path)
	}
	body := decodeBody(t, req)
	if body["srv_send_msg"] != true {
		t.Errorf("srv_send_msg = %v, want true", body["srv_send_msg"])
	}
	if body["file_type"] != float64(FileTypeVideo) {
		t.Errorf("file_type = %v, want %d", body["file_type"], FileTypeVideo)
	}
}

// TestUploadFileCompletingChunkedUpload covers the merge step, which passes the
// upload id instead of a URL.
func TestUploadFileCompletingChunkedUpload(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"file_uuid":"u","file_info":"FI","ttl":60,"raw_url":"https://cos.example.com/get"}`)

	resp, err := client.UploadC2CFile(t.Context(), "USER1", &FileUploadRequest{
		FileType: FileTypeVideo,
		FileName: "video.mp4",
		UploadID: "upload_a1b2c3d4e5f6",
	})
	if err != nil {
		t.Fatalf("UploadC2CFile: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	if body["upload_id"] != "upload_a1b2c3d4e5f6" {
		t.Errorf("upload_id = %v", body["upload_id"])
	}
	if body["file_name"] != "video.mp4" {
		t.Errorf("file_name = %v", body["file_name"])
	}
	if _, present := body["url"]; present {
		t.Error("url must be omitted when completing a chunked upload")
	}
	if resp.RawURL != "https://cos.example.com/get" {
		t.Errorf("RawURL = %q, want the presigned download link", resp.RawURL)
	}
}

// TestPrepareC2CUpload reproduces the documented pre-upload request.
func TestPrepareC2CUpload(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"upload_id":"upload_a1b2c3d4e5f6",
		"block_size":"10485760",
		"parts":[
			{"index":0,"presigned_url":"https://cos.example.com/upload?partNumber=1","block_size":"10485760"},
			{"index":1,"presigned_url":"https://cos.example.com/upload?partNumber=2","block_size":"10485760"}
		],
		"upload_config":{"concurrency":1,"retry_timeout":300,"retry_delay":1}
	}`)

	resp, err := client.PrepareC2CUpload(t.Context(), "USER1", &UploadPrepareRequest{
		FileType:      FileTypeVideo,
		FileSize:      "31457280",
		FileName:      "demo.mp4",
		MD5:           "d41d8cd98f00b204e9800998ecf8427e",
		SHA1:          "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		MD5OfFirst10M: "c4d8c5f3a2b1e0f9a8b7c6d5e4f3a2b1",
	})
	if err != nil {
		t.Fatalf("PrepareC2CUpload: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/v2/users/USER1/upload_prepare" {
		t.Errorf("path = %s, want the pre-upload address", req.Path)
	}

	body := decodeBody(t, req)
	// file_size and block_size are documented as strings, not numbers.
	if body["file_size"] != "31457280" {
		t.Errorf("file_size = %v, want the string form", body["file_size"])
	}
	if body["md5"] != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("md5 = %v", body["md5"])
	}
	if body["md5_10m"] != "c4d8c5f3a2b1e0f9a8b7c6d5e4f3a2b1" {
		t.Errorf("md5_10m = %v, want the documented json name", body["md5_10m"])
	}

	if resp.UploadID != "upload_a1b2c3d4e5f6" || resp.BlockSize != "10485760" {
		t.Errorf("resp = %+v", resp)
	}
	if len(resp.Parts) != 2 {
		t.Fatalf("Parts = %d, want 2", len(resp.Parts))
	}
	if resp.Parts[0].Index != 0 || resp.Parts[0].PresignedURL == "" {
		t.Errorf("Parts[0] = %+v", resp.Parts[0])
	}
	if resp.UploadConfig.Concurrency != 1 || resp.UploadConfig.RetryTimeout != 300 || resp.UploadConfig.RetryDelay != 1 {
		t.Errorf("UploadConfig = %+v", resp.UploadConfig)
	}
}

func TestPrepareGroupUpload(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"upload_id":"u","block_size":"1","parts":[],"upload_config":{}}`)

	if _, err := client.PrepareGroupUpload(t.Context(), "GROUP1", &UploadPrepareRequest{FileType: FileTypeFile}); err != nil {
		t.Fatalf("PrepareGroupUpload: %v", err)
	}
	if got := last(t, captured).Path; got != "/v2/groups/GROUP1/upload_prepare" {
		t.Errorf("path = %s, want the group pre-upload address", got)
	}
}

// TestFinishUploadPart reproduces the documented part completion request.
func TestFinishUploadPart(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.FinishC2CUploadPart(t.Context(), "USER1", &UploadPartFinishRequest{
		UploadID:  "upload_a1b2c3d4e5f6",
		PartIndex: 0,
		BlockSize: "10485760",
		MD5:       "c4d8c5f3a2b1e0f9a8b7c6d5e4f3a2b1",
	}); err != nil {
		t.Fatalf("FinishC2CUploadPart: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/v2/users/USER1/upload_part_finish" {
		t.Errorf("path = %s, want the part finish address", req.Path)
	}
	body := decodeBody(t, req)
	if body["upload_id"] != "upload_a1b2c3d4e5f6" {
		t.Errorf("upload_id = %v", body["upload_id"])
	}
	if body["block_size"] != "10485760" {
		t.Errorf("block_size = %v, want the string form", body["block_size"])
	}
	// part_index 0 is the documented first chunk; omitting the zero value is
	// equivalent because the platform defaults to 0.
	if _, present := body["part_index"]; present {
		t.Errorf("part_index = %v, want it omitted for the zero value", body["part_index"])
	}

	if err := client.FinishC2CUploadPart(t.Context(), "USER1", &UploadPartFinishRequest{PartIndex: 2}); err != nil {
		t.Fatalf("FinishC2CUploadPart: %v", err)
	}
	body = decodeBody(t, last(t, captured))
	if body["part_index"] != float64(2) {
		t.Errorf("part_index = %v, want 2", body["part_index"])
	}
}

func TestFinishGroupUploadPart(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.FinishGroupUploadPart(t.Context(), "GROUP1", &UploadPartFinishRequest{PartIndex: 1}); err != nil {
		t.Fatalf("FinishGroupUploadPart: %v", err)
	}
	if got := last(t, captured).Path; got != "/v2/groups/GROUP1/upload_part_finish" {
		t.Errorf("path = %s, want the group part finish address", got)
	}
}

// TestFileTypeConstantsMatchDocumentation pins the documented values, because a
// wrong value silently uploads the wrong media kind.
func TestFileTypeConstantsMatchDocumentation(t *testing.T) {
	if FileTypeImage != 1 || FileTypeVideo != 2 || FileTypeAudio != 3 || FileTypeFile != 4 {
		t.Errorf("file types = %d/%d/%d/%d, want 1/2/3/4",
			FileTypeImage, FileTypeVideo, FileTypeAudio, FileTypeFile)
	}
}

// TestUploadFileErrorIsOpenAPIError checks the documented upload failures.
func TestUploadFileErrorIsOpenAPIError(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"err_code":850031,"message":"上传文件超过大小限制","trace_id":"t"}`)

	_, err := client.UploadC2CFile(t.Context(), "USER1", &FileUploadRequest{FileType: FileTypeFile})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !IsOpenAPIError(err, 850031) {
		t.Errorf("err = %v, want the uploaded-size error code", err)
	}
}
