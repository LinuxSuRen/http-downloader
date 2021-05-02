package net

import (
	"bytes"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/linuxsuren/http-downloader/mock/mhttp"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

const fakeURL = "http://fake"

func TestRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	roundTripper := mhttp.NewMockRoundTripper(ctrl)

	client := &RetryClient{
		Client: http.Client{
			Transport: roundTripper,
		},
		MaxAttempts: 3,
	}

	mockRequest, _ := http.NewRequest(http.MethodGet, fakeURL, nil)
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		Request:    mockRequest,
		Body:       ioutil.NopCloser(bytes.NewBufferString("responseBody")),
	}
	roundTripper.EXPECT().
		RoundTrip(mockRequest).Return(mockResponse, nil)

	request, _ := http.NewRequest(http.MethodGet, fakeURL, nil)
	response, err := client.Do(request)
	fmt.Println(reflect.TypeOf(err))
	assert.Nil(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}
