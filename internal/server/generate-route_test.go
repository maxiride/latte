package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

type mockDB map[string]interface{}

func (mdb mockDB) Store(ctx context.Context, uid string, i interface{}) error {
	mdb[uid] = i
	return nil
}

func (mdb mockDB) Fetch(ctx context.Context, uid string) (interface{}, error) {
	result, exists := mdb[uid]
	if !exists {
		return nil, &NotFoundError{}
	}
	return result, nil
}

func (mdb mockDB) Ping(ctx context.Context) error {
	return nil
}

// TestHandleGenerate_Basic tests the end product PDF of a generate request.
func TestHandleGenerate_Basic(t *testing.T) {
	err := os.Chdir("../../testing")
	if err != nil {
		t.Fatalf("error while moving into testing directory: %+v", err)
	}

	type test struct {
		// Name provides a short description of the test case
		Name string

		// Name of the .pdf file in the testing pdf assets folder to test final product against
		Expectation        string
		ExpectedToPass     bool
		ExpectedStatusCode int

		// Name of .tex file in the testing tex assets folder
		TexFile         string
		TexFileRegLevel int // 0 - unregistered; 1 - registered and on disk; 2 - registered and in db and not on disk

		// Name of .json file in the testing details assets folder
		DtlsFile         string
		DtlsFileRegLevel int // 0 - unregistered; 1 - registered and on disk; 2 - registered and in db and not on disk

		// List of resource file names in the testing resources assets folder
		Resources         []string
		ResourcesRegLevel int // 0 - unregistered; 1 - registered and on disk; 2 - registered and in db and not on disk

		// Needs to have keys "left" and "right", both of which have values which are two character strings
		Delimiters map[string]string

		// OnMissingKey valid values: 'error', 'zero', 'nothing'
		OnMissingKey string
	}

	tt := []test{
		test{
			Name:           "Basic",
			TexFile:        "hello-world.tex",
			DtlsFile:       "hello-world_alice.json",
			Resources:      nil,
			Delimiters:     map[string]string{"left": "#!", "right": "!#"},
			Expectation:    "hello-world_alice.pdf",
			ExpectedToPass: true,
		},
		test{
			Name:            "Registered tex file",
			TexFile:         "hello-world.tex",
			TexFileRegLevel: 1,
			DtlsFile:        "hello-world_alice.json",
			Resources:       nil,
			Delimiters:      map[string]string{"left": "#!", "right": "!#"},
			Expectation:     "hello-world_alice.pdf",
			ExpectedToPass:  true,
		},
		test{
			Name:            "Registered tex file in db",
			TexFile:         "hello-world.tex",
			TexFileRegLevel: 2,
			DtlsFile:        "hello-world_alice.json",
			Resources:       nil,
			Delimiters:      map[string]string{"left": "#!", "right": "!#"},
			Expectation:     "hello-world_alice.pdf",
			ExpectedToPass:  true,
		},
		test{
			Name:             "Registered details file",
			TexFile:          "hello-world.tex",
			DtlsFile:         "hello-world_alice.json",
			DtlsFileRegLevel: 1,
			Resources:        nil,
			Delimiters:       map[string]string{"left": "#!", "right": "!#"},
			Expectation:      "hello-world_alice.pdf",
			ExpectedToPass:   true,
		},
		test{
			Name:             "Registered details file in db",
			TexFile:          "hello-world.tex",
			DtlsFile:         "hello-world_alice.json",
			DtlsFileRegLevel: 2,
			Resources:        nil,
			Delimiters:       map[string]string{"left": "#!", "right": "!#"},
			Expectation:      "hello-world_alice.pdf",
			ExpectedToPass:   true,
		},
		test{
			Name:           "Wrong details file",
			TexFile:        "hello-world.tex",
			DtlsFile:       "hello-world_wrong-field.json",
			Delimiters:     map[string]string{"left": "#!", "right": "!#"},
			OnMissingKey:   "error",
			Resources:      nil,
			ExpectedToPass: false,
		},
	}

	// Create temp dir for testing
	testingDir, err := ioutil.TempDir("./", "testingTmp")
	if err != nil {
		t.Fatal("error creating root testingTmp directory")
	}
	err = os.Chdir(testingDir)
	if err != nil {
		t.Fatal("error moving into testingTmp directory")
	}
	defer func() {
		os.Chdir("../")
		os.RemoveAll(testingDir)
	}()

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			// Each test case uses a new server
			s := Server{
				cmd:        "pdflatex",
				errLog:     log.New(log.Writer(), tc.Name+" Error: ", log.LstdFlags),
				infoLog:    log.New(ioutil.Discard, "", log.LstdFlags),
				tCacheSize: 1,
				rCacheSize: 1,
			}

			// Does the test case require a local directory?
			s.rootDir, err = ioutil.TempDir("./", "test_"+tc.Name)
			if err != nil {
				t.Fatalf("error while creating temporary directory: %s", err.Error())
			}
			os.Chdir(s.rootDir)
			defer func() {
				os.Chdir("../")
			}()
			// Does the test case require a mock db?
			if tc.TexFileRegLevel == 2 ||
				tc.DtlsFileRegLevel == 2 ||
				tc.ResourcesRegLevel == 2 {
				s.db = mockDB(map[string]interface{}{})
			}

			// Build up the url query and payload
			q := url.Values{}
			reqBody := struct {
				Template     string                 `json:"template"`
				Details      map[string]interface{} `json:"details"`
				Resources    map[string]string      `json:"resources"`
				Delimiters   map[string]string      `json:"delimiters, omitempty"`
				OnMissingKey string                 `json:"onMissingKey, omitempty"`
			}{
				Delimiters:   tc.Delimiters,
				OnMissingKey: tc.OnMissingKey,
			}

			// Handle Tex file
			path := "../../assets/templates/" + tc.TexFile
			fileContentsBase64, err := GetContentsBase64(path)
			if err != nil {
				wd, _ := os.Getwd()
				t.Fatalf("error while opening template file: %+v; wd: %s", err, wd)
			}
			switch tc.TexFileRegLevel {
			case 0:
				reqBody.Template = fileContentsBase64
			case 1:
				fileContents, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("error while opening details file: %+v", err)
				}
				fPath := tc.TexFile + tc.Delimiters["left"] + tc.Delimiters["right"]
				err = toDisk(fileContents, fPath)
				if err != nil {
					wd, _ := os.Getwd()
					t.Fatalf("error while writing file to disk: %s; wd: %s", err.Error(), wd)
				}
				q.Set("tmpl", tc.TexFile)
			case 2:
				fileContents, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("error while opening details file: %+v", err)
				}
				id := tc.TexFile + tc.Delimiters["left"] + tc.Delimiters["right"]
				err = s.db.Store(context.Background(), id, fileContents)
				if err != nil {
					t.Fatalf("error while saving file to db: %s", err.Error())
				}
				q.Set("tmpl", tc.TexFile)
			default:
				t.Fatalf("invalid TexFileRegLevel value")
			}

			// Handle Dtls file
			path = "../../assets/details/" + tc.DtlsFile
			fileContentsJSON, err := GetContentsJSON(path)
			if err != nil {
				t.Fatalf("error while opening template file: %+v", err)
			}
			switch tc.DtlsFileRegLevel {
			case 0:
				reqBody.Details = fileContentsJSON
			case 1:
				fileContents, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("error while opening details file: %+v", err)
				}
				fPath := tc.DtlsFile
				err = toDisk(fileContents, fPath)
				if err != nil {
					t.Fatalf("error while writing file to disk: %s", err.Error())
				}
				q.Set("dtls", tc.DtlsFile)
			case 2:
				fileContents, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("error while opening details file: %+v", err)
				}
				err = s.db.Store(context.Background(), tc.DtlsFile, fileContents)
				if err != nil {
					t.Fatalf("error while saving file to db: %s", err.Error())
				}
				q.Set("dtls", tc.DtlsFile)
			default:
				t.Fatalf("invalid DtlsRegLevel value")
			}

			// Handle Resource files
			switch tc.ResourcesRegLevel {
			case 0:
				resources := make(map[string]string)
				for _, rn := range tc.Resources {
					path := "../../assets/resources/" + rn
					resource, err := GetContentsBase64(path)
					if err != nil {
						t.Fatalf("error while opening resource file: %+v", err)
					}
					resources[rn] = resource
				}
				reqBody.Resources = resources
			case 1:
				for _, fileName := range tc.Resources {
					path = "../../assets/resources/" + fileName
					fileContents, err := ioutil.ReadFile(path)
					if err != nil {
						t.Fatalf("error while opening details file: %+v", err)
					}
					err = toDisk(fileContents, fileName)
					if err != nil {
						t.Fatalf("error while writing file to disk: %s", err.Error())
					}
					q.Set("rsc", fileName)
				}
			case 2:
				for _, fileName := range tc.Resources {
					path = "../../assets/resources/" + fileName
					fileContents, err := ioutil.ReadFile(path)
					if err != nil {
						t.Fatalf("error while opening details file: %+v", err)
					}
					err = s.db.Store(context.Background(), fileName, fileContents)
					if err != nil {
						t.Fatalf("error while saving file to mock db: %s", err.Error())
					}
					q.Set("rsc", fileName)
				}

			default:
				t.Fatalf("invalid ResourcesRegLevel value")
			}

			// Create request and ResponseWriter recorded
			testPayload, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("error while creating request payload: %+v", err)
			}
			req := httptest.NewRequest("GET", "/generate", bytes.NewBuffer(testPayload))
			req.Header.Set("Content-Type", "application/json")
			req.URL.RawQuery = q.Encode()
			rr := httptest.NewRecorder()

			// Create the HTTP handler to be tested and save current working directory to move back into
			// after handler being tested is called; this is necessary since the handler changes the current working directory.
			wd, err := os.Getwd()
			if err != nil {
				t.Fatalf("error while grabbing current directory: %+v", err)
			}
			hgFunc, err := s.handleGenerate()
			if err != nil {
				t.Fatalf("error while creating the function being tested: %+v", err)
			}
			os.Chdir("../")
			hgFunc(rr, req)
			err = os.Chdir(wd)
			if err != nil {
				t.Fatalf("error while moving back into testing directory")
			}
			response := rr.Result()
			if response.StatusCode != 200 && tc.ExpectedToPass {
				responseBody, err := ioutil.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					t.Fatalf("unable to read response body")
				}
				t.Fatalf(`Got non 200 status from result: {"status": %q, "response_body": %q}`, response.Status, string(responseBody))
			}

			// If test case is expected to pass, grab expected PDF to test against and compare it to the received PDF
			if tc.ExpectedToPass {
				path := "../../assets/PDFs/" + tc.Expectation
				expectedPDF, err := GetContentsBase64(path)
				if err != nil {
					t.Fatalf("error while reading expected PDF: %+v", err)
				}
				receivedPDF, err := ioutil.ReadAll(response.Body)
				if err != nil {
					t.Fatalf("error while reading received PDF: %+v", err)
				}
				response.Body.Close()
				receivedPDF64 := base64.StdEncoding.EncodeToString(receivedPDF)

				// Since PDFs seem to have some 'wiggle' to them, we have to make do with checking if our PDFs are 'close enough'
				// (We define 'close enough' as no more than 1% difference when comparing byte-by-byte)
				errorRate := DiffP(receivedPDF64, expectedPDF, t)
				if errorRate > 1.0 {
					t.Errorf("mismatch between received pdf and expected pdf exceeded 1%%: %f%%", errorRate)
				}
			} else if response.StatusCode == 200 {
				t.Errorf("expected non 200 status code\n")
			}
		})
	}
}

func GetContentsBase64(path string) (string, error) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return "", err
	}
	fbytes, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	estring := base64.StdEncoding.EncodeToString(fbytes)
	return estring, nil
}

func GetContentsJSON(path string) (map[string]interface{}, error) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	data := make(map[string]interface{})
	err = json.NewDecoder(f).Decode(&data)
	return data, err
}

// DiffP tests the equality of the two strings and returns the percentage by which they differ.
func DiffP(received, expected string, t *testing.T) float32 {
	if len(received) != len(expected) {
		t.Fatalf("Received PDF differs from expected PDF: received length = %d \t expected length = %d",
			len(received), len(expected))
	}
	var mismatches int
	for i, c := range received {
		if byte(c) != byte(expected[i]) {
			mismatches++
		}
	}
	errorRate := float32(mismatches) / float32(len(expected))
	errorRate *= 100
	return errorRate
}
