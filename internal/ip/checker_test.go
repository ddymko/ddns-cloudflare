package ip

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	intHTTP "github.com/devbytes-cloud/dynamic-cf/internal/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

var _ intHTTP.ClientInterface = &MockClient{}

func TestGetIP(t *testing.T) {
	tt := map[string]struct {
		ip             string
		mockResp       *http.Response
		mockError      error
		wantIP         string
		wantErr        bool
		wantErrMessage string
	}{
		"success": {
			mockResp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("1.1.1.1")),
			},
			mockError: nil,
			wantIP:    "1.1.1.1",
			wantErr:   false,
		},
		"failure: status code not 200": {
			mockResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("")),
			},
			mockError:      nil,
			wantIP:         "",
			wantErr:        true,
			wantErrMessage: "received status code 500",
		},
		"failure: on call": {
			mockResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("")),
			},
			mockError:      fmt.Errorf("failure"),
			wantIP:         "",
			wantErr:        true,
			wantErrMessage: "failure",
		},
	}

	for k, v := range tt {
		t.Run(k, func(t *testing.T) {
			mockClient := new(MockClient)
			mockLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
			checker := NewChecker(context.Background(), nil, mockLogger, mockClient)

			mockClient.
				On("Get", "https://ifconfig.me/ip").
				Return(v.mockResp, v.mockError)

			ip, err := checker.getIP()

			if v.wantErr {
				assert.EqualError(t, err, v.wantErrMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, v.wantIP, ip)
			}
		})
	}
}

func TestHasIPChanged(t *testing.T) {
	hasIPChanged("1.1.1.1")
}

func TestLastKnownIP(t *testing.T) {
	a, b := lastKnownIP()
	if b != nil {
		panic(b)
	}

	fmt.Println(a)
}
