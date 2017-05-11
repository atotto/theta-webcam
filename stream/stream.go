package stream

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type Command struct {
	Name       string `json:"name"`
	Parameters struct {
		SessionID string `json:"sessionId,omitempty"`
		Options   struct {
			Iso                  int `json:"iso,omitempty"`
			ExposureCompensation int `json:"exposureCompensation,omitempty"`
		} `json:"options,omitempty"`
	} `json:"parameters,omitempty"`
}

func newCommand(name string) *Command {
	return &Command{
		Name: name,
	}
}

func (cmd *Command) SetSessionID(id string) *Command {
	cmd.Parameters.SessionID = id
	return cmd
}

type CommandResponse struct {
	Name    string `json:"name"`
	Results struct {
		SessionID string `json:"sessionId,omitempty"`
	} `json:"results,omitempty"`
}

var (
	start = []byte{0xFF, 0xD8}
	end   = []byte{0xFF, 0xD9}
)

type server struct {
	reader io.ReadCloser
	data   []byte
}

func NewLiveStream(endpoint string) (*server, error) {
	r, err := newStream(endpoint)
	if err != nil {
		return nil, err
	}

	return &server{
		reader: r,
		data:   make([]byte, 0, 60000),
	}, nil
}

func newStream(endpoint string) (io.ReadCloser, error) {
	client := &http.Client{Timeout: 0}

	buf, err := json.Marshal(newCommand("camera.startSession"))
	if err != nil {
		return nil, err
	}
	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	res := &CommandResponse{}
	err = json.NewDecoder(resp.Body).Decode(res)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	buf, err = json.Marshal(newCommand("camera._getLivePreview").SetSessionID(res.Results.SessionID))
	if err != nil {
		return nil, err
	}
	resp, err = client.Post(endpoint, "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *server) NextImage() (jpeg []byte, err error) {
	buf := make([]byte, 2)
	var reading bool
	for {
		n, err := s.reader.Read(buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}

		if !reading && bytes.Equal(buf, start) {
			s.data = s.data[:0]
			reading = true
		}
		if reading {
			s.data = append(s.data, buf...)
		}
		if reading && bytes.Equal(buf, end) {
			reading = false
			return s.data, nil
		}
	}
}
