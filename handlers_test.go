package trama

import (
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestServeWeb(t *testing.T) {
	data := []struct {
		description string

		contentGet        string
		dataGet           interface{}
		expectedResultGet string
		redirectURL       string

		contentPost        string
		dataPost           interface{}
		expectedResultPost string

		interceptors   WebInterceptorChain
		testStatusCode bool
	}{
		{
			description: "It should write the expected results",
			contentGet: `
				Um {{.Galo}} sozinho não tece uma manhã:
				ele precisará sempre de outros {{.Galos}}.
				De um que {{myFunc "apanhe"}} esse grito que ele
				e o lance a outro; de um outro {{.Galo}}
				que apanhe o grito de um {{.Galo}} antes
				e o lance a outro; e de outros {{.Galos}}
				que com muitos outros {{.Galos}} se cruzem
				os fios de sol de seus gritos de {{.Galo}},
				para que a manhã, desde uma teia tênue,
				se vá tecendo, entre todos os {{.Galos}}.`,
			dataGet: struct{ Galo, Galos string }{"galo", "galos"},
			expectedResultGet: `
				Um galo sozinho não tece uma manhã:
				ele precisará sempre de outros galos.
				De um que !confidential! esse grito que ele
				e o lance a outro; de um outro galo
				que apanhe o grito de um galo antes
				e o lance a outro; e de outros galos
				que com muitos outros galos se cruzem
				os fios de sol de seus gritos de galo,
				para que a manhã, desde uma teia tênue,
				se vá tecendo, entre todos os galos.`,
			contentPost: `
				E se encorpando em tela, entre {{.Todos}},
				se erguendo tenda, onde entrem {{.Todos}},
				se entretendo para {{.Todos}}, no toldo
				(a manhã) que plana livre de armação.
				A manhã, toldo de um tecido tão aéreo
				que, tecido, se eleva por si: luz balão.`,
			dataPost: struct{ Todos string }{"todos"},
			expectedResultPost: `
				E se encorpando em tela, entre todos,
				se erguendo tenda, onde entrem todos,
				se entretendo para todos, no toldo
				(a manhã) que plana livre de armação.
				A manhã, toldo de um tecido tão aéreo
				que, tecido, se eleva por si: luz balão.`,
			testStatusCode: true,
		},
		{
			description: "It should write the expected results after running the interceptors",
			contentGet: `
				Um {{.Galo}} sozinho não tece uma manhã:
				ele precisará sempre de outros {{.Galos}}.
				De um que apanhe esse grito que ele
				e o lance a outro; de um outro {{.Galo}}
				que apanhe o grito de um {{.Galo}} antes
				e o lance a outro; e de outros {{.Galos}}
				que com muitos outros {{.Galos}} se cruzem
				os fios de sol de seus gritos de {{.Galo}},
				para que a manhã, desde uma teia tênue,
				se vá tecendo, entre todos os {{.Galos}}.`,
			dataGet: struct{ Galo, Galos string }{"galo", "galos"},
			expectedResultGet: `
				Um galo sozinho não tece uma manhã:
				ele precisará sempre de outros galos.
				De um que apanhe esse grito que ele
				e o lance a outro; de um outro galo
				que apanhe o grito de um galo antes
				e o lance a outro; e de outros galos
				que com muitos outros galos se cruzem
				os fios de sol de seus gritos de galo,
				para que a manhã, desde uma teia tênue,
				se vá tecendo, entre todos os galos.`,
			contentPost: `
				E se encorpando em tela, entre {{.Todos}},
				se erguendo tenda, onde entrem {{.Todos}},
				se entretendo para {{.Todos}}, no toldo
				(a manhã) que plana livre de armação.
				A manhã, toldo de um tecido tão aéreo
				que, tecido, se eleva por si: luz balão.`,
			dataPost: struct{ Todos string }{"todos"},
			expectedResultPost: `
				E se encorpando em tela, entre todos,
				se erguendo tenda, onde entrem todos,
				se entretendo para todos, no toldo
				(a manhã) que plana livre de armação.
				A manhã, toldo de um tecido tão aéreo
				que, tecido, se eleva por si: luz balão.`,
			testStatusCode: true,
			interceptors: WebInterceptorChain{
				&struct{ NopWebInterceptor }{},
				&struct{ NopWebInterceptor }{},
				&struct{ NopWebInterceptor }{},
			},
		},
		{
			description:       "It should break at the interceptor's Before run",
			contentGet:        "Tecendo a manhã",
			expectedResultGet: "",
			interceptors: WebInterceptorChain{
				&struct{ NopWebInterceptor }{},
				&brokenBeforeInterceptor{},
				&struct{ NopWebInterceptor }{},
			},
		},
		{
			description:       "It should break at the interceptor's After run",
			contentGet:        "Tecendo a manhã",
			expectedResultGet: "",
			interceptors: WebInterceptorChain{
				&struct{ NopWebInterceptor }{},
				&brokenAfterInterceptor{},
				&struct{ NopWebInterceptor }{},
			},
			testStatusCode: true,
		},
		{
			description:       "It should redirect when necessary",
			redirectURL:       "/test",
			expectedResultGet: "<a href=\"/test\">Found</a>.\n\n",
			testStatusCode:    false,
		},
	}

	for i, item := range data {
		mock := &mockWebHandler{
			templateGetContent:     item.contentGet,
			templateGetData:        item.dataGet,
			templateGetRedirectURL: item.redirectURL,
			templatePostContent:    item.contentPost,
			templatePostData:       item.dataPost,
			interceptors:           item.interceptors,
		}

		defer mock.closeTemplates()
		templatesNames := mock.Templates()
		templ, err := template.New("mock").Funcs(mock.TemplatesFunc()).ParseFiles(templatesNames...)

		if err != nil {
			t.Fatalf("Item %d, “%s”, unexpected error: “%s”", i, item.description, err)
		}

		handler := adapter{
			webHandler: func() WebHandler { return mock },
			err: func(err error) {
				notBeforeError := err.Error() != brokenBeforeError.Error()
				notAfterError := err.Error() != brokenAfterError.Error()

				if notBeforeError && notAfterError {
					t.Errorf("Item %d, “%s”, unexpected error: “%s”", i, item.description, err)
				}
			},
			template: templ,
		}

		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/uri", nil)

		if err != nil {
			t.Error(err)
		}

		handler.serveHTTP(w, r)

		if item.testStatusCode && w.Code != http.StatusOK {
			t.Errorf("Item %d, “%s”, wrong status code. Expecting 200; found %d", i, item.description, w.Code)

		} else if item.redirectURL != "" && w.Code != http.StatusFound {
			t.Errorf("Item %d, “%s”, wrong status code. Expecting 302; found %d", i, item.description, w.Code)
		}

		if w.Body.String() != item.expectedResultGet {
			t.Errorf("Item %d, “%s”, unexpected result. Expecting “%s”;\nfound “%s”", i, item.description, item.expectedResultGet, w.Body.String())
		}

		w = httptest.NewRecorder()
		r, err = http.NewRequest("POST", "/uri", nil)

		if err != nil {
			t.Error(err)
		}

		handler.serveHTTP(w, r)

		if item.testStatusCode && w.Code != http.StatusOK {
			t.Errorf("Item %d, “%s”, wrong status code. Expecting 200; found %d", i, item.description, w.Code)
		}

		if w.Body.String() != item.expectedResultPost {
			t.Errorf("Item %d, “%s”, unexpected result. Expecting “%s”;\nfound “%s”", i, item.description, item.expectedResultPost, w.Body.String())
		}

		if item.testStatusCode && w.Header().Get("Set-Cookie") != "cookie1=value1" {
			t.Errorf("Item %d, “%s”, unexpected result. Expecting “cookie1=value1”;\nfound “%s”", i, item.description, w.Header().Get("Set-Cookie"))
		}

		w = httptest.NewRecorder()
		r, err = http.NewRequest("DELETE", "/uri", nil)

		if err != nil {
			t.Error(err)
		}

		handler.serveHTTP(w, r)

		if item.testStatusCode && w.Code != http.StatusNotImplemented {
			t.Errorf("Item %d, “%s”, wrong status code. Expecting %d; found %d", i, item.description, http.StatusNotImplemented, w.Code)
		}
	}
}

func TestServeAJAX(t *testing.T) {
	data := []struct {
		description           string
		interceptors          AJAXInterceptorChain
		httpMethod            string
		expectedStatusCode    int
		handlerShouldBeCalled bool
	}{
		{
			description:           "It should handle the GET request properly",
			httpMethod:            "GET",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:           "It should handle the PUT request properly",
			httpMethod:            "PUT",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:           "It should handle the POST request properly",
			httpMethod:            "POST",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:           "It should handle the PATCH request properly",
			httpMethod:            "PATCH",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:           "It should handle the DELETE request properly",
			httpMethod:            "DELETE",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:           "It should handle the HEAD request properly",
			httpMethod:            "HEAD",
			expectedStatusCode:    http.StatusOK,
			handlerShouldBeCalled: true,
		},
		{
			description:        "It should handle the HEAD request with interceptors properly",
			httpMethod:         "HEAD",
			expectedStatusCode: http.StatusOK,
			interceptors: AJAXInterceptorChain{
				&struct{ NopAJAXInterceptor }{},
				&struct{ NopAJAXInterceptor }{},
				&struct{ NopAJAXInterceptor }{},
			},
			handlerShouldBeCalled: true,
		},
		{
			description:        "It should break at the interceptor's Before run and not run the handler's method",
			httpMethod:         "HEAD",
			expectedStatusCode: http.StatusInternalServerError,
			interceptors: AJAXInterceptorChain{
				&struct{ NopAJAXInterceptor }{},
				&brokenBeforeAJAXInterceptor{},
				&struct{ NopAJAXInterceptor }{},
			},
			handlerShouldBeCalled: false,
		},
	}

	for i, item := range data {
		handleFuncCalled := false
		mock := &mockAJAXHandler{
			handleFunc: func(http.ResponseWriter, *http.Request) {
				handleFuncCalled = true
			},
			interceptors: item.interceptors,
		}
		handler := adapter{
			ajaxHandler: func() AJAXHandler { return mock },
			err: func(err error) {
				t.Errorf("Item %d, “%s”, unexpected error found: %s", i, item.description, err)
			},
			uriVars: map[string]string{"param1": "1", "param2": "2"},
		}

		w := httptest.NewRecorder()
		r, err := http.NewRequest(item.httpMethod, "", nil)

		if err != nil {
			t.Error(err)
		}

		handler.serveHTTP(w, r)

		if item.handlerShouldBeCalled {
			if !handleFuncCalled {
				t.Errorf("Item %d, “%s”, not calling handler", i, item.description)
			} else {
				if mock.methodCalled != item.httpMethod {
					t.Errorf("Item %d, “%s”, wrong method called. Expecting %s; found %s", i, item.description, item.httpMethod, mock.methodCalled)
				}
			}
		}

		if mock.Param1 != "1" {
			t.Errorf("Item %d, “%s”, wrong param1. Expecting “1”; found “%s”", i, item.description, mock.Param1)
		}

		if mock.Param2 != 2 {
			t.Errorf("Item %d, “%s”, wrong param1. Expecting “2”; found “%d”", i, item.description, mock.Param2)
		}

		if w.Code != item.expectedStatusCode {
			t.Errorf("Item %d, “%s”, wrong status code. Expecting %d; found %d", i, item.description, item.expectedStatusCode, w.Code)
		}
	}
}

type mockWebHandler struct {
	templateGet            *os.File
	templateGetContent     string
	templateGetData        interface{}
	templateGetRedirectURL string

	templatePost        *os.File
	templatePostContent string
	templatePostData    interface{}

	interceptors WebInterceptorChain
}

func (m *mockWebHandler) closeTemplates() {
	m.templateGet.Close()
	m.templatePost.Close()
}

func (m *mockWebHandler) Get(res Response, req *http.Request) error {
	if m.templateGetRedirectURL != "" {
		res.Redirect(m.templateGetRedirectURL, http.StatusFound)

	} else if m.templateGet == nil {
		return errors.New("Template GET not set")
	}

	res.SetTemplate(m.templateGet.Name(), m.templateGetData)
	return nil
}

func (m *mockWebHandler) Post(res Response, req *http.Request) error {
	if m.templatePost == nil {
		return errors.New("Template POST not set")
	}

	res.SetCookie(&http.Cookie{
		Name:  "cookie1",
		Value: "value1",
	})
	res.SetTemplate(m.templatePost.Name(), m.templatePostData)
	return nil
}

func (m *mockWebHandler) Templates() []string {
	var err error

	m.templateGet, err = ioutil.TempFile("", "mockWebHandler")
	if err != nil {
		println(err.Error())
		return nil
	}

	m.templatePost, err = ioutil.TempFile("", "mockWebHandler")
	if err != nil {
		println(err.Error())
		return nil
	}

	if _, err = io.WriteString(m.templateGet, m.templateGetContent); err != nil {
		println(err.Error())
		return nil
	}

	if _, err = io.WriteString(m.templatePost, m.templatePostContent); err != nil {
		println(err.Error())
		return nil
	}

	return []string{m.templateGet.Name(), m.templatePost.Name()}
}

func (m *mockWebHandler) Interceptors() WebInterceptorChain {
	return m.interceptors
}

func (m *mockWebHandler) TemplatesFunc() template.FuncMap {
	return template.FuncMap{
		"myFunc": func(value string) string {
			return "!confidential!"
		},
	}
}

type brokenBeforeInterceptor struct {
	NopWebInterceptor
}

var (
	brokenBeforeError = errors.New("Error from a broken Before implementation of a web interceptor")
	brokenAfterError  = errors.New("Error from a broken After implementation of a web interceptor")
)

func (b *brokenBeforeInterceptor) Before(Response, *http.Request) error {
	return brokenBeforeError
}

type brokenAfterInterceptor struct {
	NopWebInterceptor
}

func (b *brokenAfterInterceptor) After(Response, *http.Request) error {
	return brokenAfterError
}

type mockAJAXHandler struct {
	Param1       string `param:"param1"`
	Param2       int    `param:"param2"`
	handleFunc   func(http.ResponseWriter, *http.Request)
	interceptors AJAXInterceptorChain
	methodCalled string
}

func (m *mockAJAXHandler) Get(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "GET"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Post(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "POST"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Put(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "PUT"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Delete(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "DELETE"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Patch(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "PATCH"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Head(w http.ResponseWriter, r *http.Request) {
	m.methodCalled = "HEAD"
	m.handleFunc(w, r)
}

func (m *mockAJAXHandler) Interceptors() AJAXInterceptorChain {
	return m.interceptors
}

type brokenBeforeAJAXInterceptor struct {
	NopAJAXInterceptor
}

func (b *brokenBeforeAJAXInterceptor) Before(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

type brokenAfterAJAXInterceptor struct {
	NopAJAXInterceptor
}

func (b *brokenAfterAJAXInterceptor) After(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}
