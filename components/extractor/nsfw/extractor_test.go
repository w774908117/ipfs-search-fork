package nsfw

import (
	"context"
	"net/http"
	"testing"

	"github.com/dankinder/httpmock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/ipfs-search/ipfs-search/components/extractor"
	indexTypes "github.com/ipfs-search/ipfs-search/components/index/types"

	"github.com/ipfs-search/ipfs-search/instr"
	t "github.com/ipfs-search/ipfs-search/types"
)

const testCID = "QmehHHRh1a7u66r7fugebp6f6wGNMGCa7eho9cgjwhAcm2"

type NSFWTestSuite struct {
	suite.Suite

	ctx context.Context
	e   extractor.Extractor

	cfg *Config

	mockAPIHandler *httpmock.MockHandler
	mockAPIServer  *httpmock.Server
	responseHeader http.Header
}

func (s *NSFWTestSuite) SetupTest() {
	s.ctx = context.Background()

	s.mockAPIHandler = &httpmock.MockHandler{}
	s.mockAPIServer = httpmock.NewServer(s.mockAPIHandler)
	s.responseHeader = http.Header{
		"Content-Type": []string{"application/json"},
	}

	s.cfg = DefaultConfig()
	s.cfg.NSFWServerURL = s.mockAPIServer.URL()

	s.e = New(s.cfg, http.DefaultClient, instr.New())
}

func (s *NSFWTestSuite) TearDownTest() {
	s.mockAPIServer.Close()
}

func (s NSFWTestSuite) TestExtract() {
	testJSON := []byte(`
		{
		  "classification": {
		    "neutral": 0.9980410933494568,
		    "drawing": 0.001135041005909443,
		    "porn": 0.00050011818530038,
		    "hentai": 0.00016194644558709115,
		    "sexy": 0.00016178081568796188
		  },
		  "nsfwjsVersion": "2.4.1"
		}
    `)

	r := &t.AnnotatedResource{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol,
			ID:       testCID,
		},
		Stat: t.Stat{
			Size: 400,
		},
	}

	extractorURL := "/classify/" + testCID

	s.mockAPIHandler.
		On("Handle", "GET", extractorURL, mock.Anything).
		Return(httpmock.Response{
			Body: testJSON,
		}).
		Once()

	f := &indexTypes.File{
		Document: indexTypes.Document{
			Size: r.Size,
		},
	}

	err := s.e.Extract(s.ctx, r, &f)

	s.NoError(err)
	s.mockAPIHandler.AssertExpectations(s.T())

	s.Equal(f.NSFW.Classification.Neutral, 0.9980410933494568)
	s.Equal(f.NSFW.Classification.Drawing, 0.001135041005909443)
	s.Equal(f.NSFW.Classification.Porn, 0.00050011818530038)
	s.Equal(f.NSFW.Classification.Hentai, 0.00016194644558709115)
	s.Equal(f.NSFW.Classification.Sexy, 0.00016178081568796188)
	s.Equal(f.NSFW.NSFWVersion, "2.4.1")
}

func (s NSFWTestSuite) TestExtractMaxFileSize() {
	s.cfg.MaxFileSize = 100
	s.e = New(s.cfg, http.DefaultClient, instr.New())

	r := &t.AnnotatedResource{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol,
			ID:       testCID,
		},
		Stat: t.Stat{
			Size: uint64(s.cfg.MaxFileSize + 1),
		},
	}

	f := &indexTypes.File{}
	err := s.e.Extract(s.ctx, r, &f)

	s.Error(err, extractor.ErrFileTooLarge)
	s.mockAPIHandler.AssertExpectations(s.T())
}

func (s NSFWTestSuite) TestExtractUpstreamError() {
	r := &t.AnnotatedResource{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol,
			ID:       testCID,
		},
	}

	// Closing server early, generates a request error.
	s.mockAPIServer.Close()

	f := &indexTypes.File{}

	err := s.e.Extract(s.ctx, r, &f)
	s.Error(err, extractor.ErrRequest)
}

func (s NSFWTestSuite) TestServer500() {
	// 500 will just propagate whatever error we're getting from a lower level
	r := &t.AnnotatedResource{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol,
			ID:       testCID,
		},
	}

	extractorURL := "/classify/" + testCID

	s.mockAPIHandler.
		On("Handle", "GET", extractorURL, mock.Anything).
		Return(httpmock.Response{
			Status: 500,
			Body:   []byte("{}"),
		}).
		Once()

	f := &indexTypes.File{}

	err := s.e.Extract(s.ctx, r, &f)

	s.Error(err, extractor.ErrUnexpectedResponse)
	s.mockAPIHandler.AssertExpectations(s.T())
}

func (s NSFWTestSuite) TestExtractInvalidJSON() {
	testJSON := []byte(`invalid JSON`)

	r := &t.AnnotatedResource{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol,
			ID:       testCID,
		},
		Stat: t.Stat{
			Size: 400,
		},
	}

	extractorURL := "/classify/" + testCID

	s.mockAPIHandler.
		On("Handle", "GET", extractorURL, mock.Anything).
		Return(httpmock.Response{
			Body: testJSON,
		}).
		Once()

	f := &indexTypes.File{
		Document: indexTypes.Document{
			Size: r.Size,
		},
	}

	err := s.e.Extract(s.ctx, r, &f)

	s.Error(err, extractor.ErrUnexpectedResponse)
	s.mockAPIHandler.AssertExpectations(s.T())
}

func TestNSFWTestSuite(t *testing.T) {
	suite.Run(t, new(NSFWTestSuite))
}
