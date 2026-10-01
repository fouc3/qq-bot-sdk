package qqbotsdk

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// Rich media file types, as documented.
//
// Exceeding the soft limit downgrades the upload to a plain file; exceeding the
// hard limit (200 MB) fails.
const (
	// FileTypeImage is png or jpg, soft limit 20 MB.
	FileTypeImage = 1
	// FileTypeVideo is mp4, soft limit 30 MB.
	FileTypeVideo = 2
	// FileTypeAudio is silk, soft limit 20 MB.
	FileTypeAudio = 3
	// FileTypeFile is any format, soft limit 200 MB.
	FileTypeFile = 4
)

// FileUploadRequest is the body of a rich media upload.
//
// Either URL transfers a file the platform can download, or UploadID completes
// a chunked upload that PrepareUpload started.
type FileUploadRequest struct {
	// FileType is one of the FileType constants.
	FileType int `json:"file_type,omitempty"`
	// URL is the publicly reachable file address. May be empty when UploadID
	// is used.
	URL string `json:"url,omitempty"`
	// SrvSendMsg sends the message as part of the upload, consuming an active
	// message quota. When false only a FileInfo is returned.
	SrvSendMsg bool `json:"srv_send_msg,omitempty"`
	// FileName is the optional file name.
	FileName string `json:"file_name,omitempty"`
	// UploadID completes a chunked upload started by PrepareUpload.
	UploadID string `json:"upload_id,omitempty"`
}

// FileUploadResponse is the result of a rich media upload.
type FileUploadResponse struct {
	// FileUUID is the unique file id.
	FileUUID string `json:"file_uuid"`
	// FileInfo goes into Message.Media when sending.
	FileInfo string `json:"file_info"`
	// TTL is how long FileInfo stays valid, in seconds. 0 means it does not
	// expire.
	TTL int `json:"ttl"`
	// ID is the sent message id, present only when SrvSendMsg was true.
	ID string `json:"id,omitempty"`
	// RawURL is a presigned download URL, returned only when completing a
	// chunked upload of an image, video or audio file.
	RawURL string `json:"raw_url,omitempty"`
}

// UploadC2CFile uploads rich media for a single chat.
//
// Files uploaded here can only be sent to single chats; the group endpoint is
// separate.
func (c *Client) UploadC2CFile(ctx context.Context, userOpenID string, req *FileUploadRequest) (*FileUploadResponse, error) {
	if userOpenID == "" {
		return nil, errors.New("qqbotsdk: UploadC2CFile needs a user openid")
	}
	var out FileUploadResponse
	path := "/v2/users/" + url.PathEscape(userOpenID) + "/files"
	if err := c.doJSON(ctx, http.MethodPost, path, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadGroupFile uploads rich media for a group chat.
func (c *Client) UploadGroupFile(ctx context.Context, groupOpenID string, req *FileUploadRequest) (*FileUploadResponse, error) {
	if groupOpenID == "" {
		return nil, errors.New("qqbotsdk: UploadGroupFile needs a group openid")
	}
	var out FileUploadResponse
	path := "/v2/groups/" + url.PathEscape(groupOpenID) + "/files"
	if err := c.doJSON(ctx, http.MethodPost, path, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadPrepareRequest starts a chunked upload.
//
// The checksums are computed over the whole file; MD5OfFirst10M covers the first
// 10002432 bytes and lets the platform recognise a file it already has.
type UploadPrepareRequest struct {
	// FileType is one of the FileType constants.
	FileType int `json:"file_type"`
	// FileSize is the total size in bytes, as a string.
	FileSize string `json:"file_size"`
	// FileName is the file name.
	FileName string `json:"file_name"`
	// MD5 is the MD5 of the whole file, in hex.
	MD5 string `json:"md5"`
	// SHA1 is the SHA1 of the whole file, in hex.
	SHA1 string `json:"sha1"`
	// MD5OfFirst10M is the MD5 of the first 10002432 bytes, in hex.
	MD5OfFirst10M string `json:"md5_10m"`
}

// UploadPart is one chunk of a prepared upload.
type UploadPart struct {
	// Index counts from 0.
	Index int `json:"index"`
	// PresignedURL receives the chunk through an HTTP PUT.
	PresignedURL string `json:"presigned_url"`
	// BlockSize is the size of this chunk in bytes, as a string.
	BlockSize string `json:"block_size"`
}

// UploadConfig is the transfer policy the platform asks the client to follow.
type UploadConfig struct {
	// Concurrency is the number of chunks to upload at once.
	Concurrency int `json:"concurrency"`
	// RetryTimeout is the retry window in seconds.
	RetryTimeout int `json:"retry_timeout"`
	// RetryDelay is the delay between retries in seconds.
	RetryDelay int `json:"retry_delay"`
}

// UploadPrepareResponse describes how to upload the chunks of a file.
type UploadPrepareResponse struct {
	// UploadID identifies the upload, and is passed to the completing call.
	UploadID string `json:"upload_id"`
	// BlockSize is the default chunk size in bytes, as a string.
	BlockSize string `json:"block_size"`
	// Parts are the chunks to upload.
	Parts []UploadPart `json:"parts"`
	// UploadConfig is the transfer policy.
	UploadConfig UploadConfig `json:"upload_config"`
}

// PrepareC2CUpload starts a chunked upload for a single chat.
func (c *Client) PrepareC2CUpload(ctx context.Context, userOpenID string, req *UploadPrepareRequest) (*UploadPrepareResponse, error) {
	if userOpenID == "" {
		return nil, errors.New("qqbotsdk: PrepareC2CUpload needs a user openid")
	}
	return c.prepareUpload(ctx, "/v2/users/", userOpenID, req)
}

// PrepareGroupUpload starts a chunked upload for a group chat.
func (c *Client) PrepareGroupUpload(ctx context.Context, groupOpenID string, req *UploadPrepareRequest) (*UploadPrepareResponse, error) {
	if groupOpenID == "" {
		return nil, errors.New("qqbotsdk: PrepareGroupUpload needs a group openid")
	}
	return c.prepareUpload(ctx, "/v2/groups/", groupOpenID, req)
}

// prepareUpload calls one of the two upload_prepare endpoints.
func (c *Client) prepareUpload(ctx context.Context, prefix, openID string, req *UploadPrepareRequest) (*UploadPrepareResponse, error) {
	var out UploadPrepareResponse
	path := prefix + url.PathEscape(openID) + "/upload_prepare"
	if err := c.doJSON(ctx, http.MethodPost, path, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadPartFinishRequest reports one chunk as uploaded.
type UploadPartFinishRequest struct {
	// UploadID is the id returned by the prepare call.
	UploadID string `json:"upload_id,omitempty"`
	// PartIndex is the chunk index that finished.
	PartIndex int `json:"part_index,omitempty"`
	// BlockSize is the chunk size in bytes, as a string.
	BlockSize string `json:"block_size,omitempty"`
	// MD5 is the chunk MD5, in hex.
	MD5 string `json:"md5,omitempty"`
}

// FinishC2CUploadPart reports one uploaded chunk of a single-chat upload.
func (c *Client) FinishC2CUploadPart(ctx context.Context, userOpenID string, req *UploadPartFinishRequest) error {
	if userOpenID == "" {
		return errors.New("qqbotsdk: FinishC2CUploadPart needs a user openid")
	}
	return c.finishUploadPart(ctx, "/v2/users/", userOpenID, req)
}

// FinishGroupUploadPart reports one uploaded chunk of a group upload.
func (c *Client) FinishGroupUploadPart(ctx context.Context, groupOpenID string, req *UploadPartFinishRequest) error {
	if groupOpenID == "" {
		return errors.New("qqbotsdk: FinishGroupUploadPart needs a group openid")
	}
	return c.finishUploadPart(ctx, "/v2/groups/", groupOpenID, req)
}

// finishUploadPart calls one of the two upload_part_finish endpoints.
func (c *Client) finishUploadPart(ctx context.Context, prefix, openID string, req *UploadPartFinishRequest) error {
	path := prefix + url.PathEscape(openID) + "/upload_part_finish"
	return c.doJSON(ctx, http.MethodPost, path, req, nil, openAPICall)
}
