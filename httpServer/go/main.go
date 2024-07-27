package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
)

const PORT = ":1437"

type Server struct {
	Listener net.Listener
}

type Req struct {
	Method  string
	URI     string
	Headers map[string]string
	Body    string
}

type Router struct {
	Conn net.Conn
	Req  *Req
}

type Service struct {
	Router *Router
	Store  map[string]string
}

func main() {
	s := NewServer()
	defer s.Listener.Close()
	log.Printf("Server is running on 0.0.0.0%s", PORT)

	for {
		conn, err := s.Listener.Accept()
		log.Println("Incoming connection from", conn.RemoteAddr())
		if err != nil {
			if err != io.EOF {
				log.Println("Error when accepting incoming connection", err)
			}
		}

		go handleAccept(conn)

	}
}

func handleAccept(conn net.Conn) {
	req, err := NewRequest(conn)
	if err != nil {
		log.Println(err)
		return
	}

	router := NewRouter(conn, req)
	router.Mux()
}

func NewRequest(conn net.Conn) (*Req, error) {
	var method, uri, reqBody string
	headers := make(map[string]string)

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSuffix(line, "\r\n")
	sliceOfLine := strings.Split(strings.TrimSpace(line), " ")

	// method
	method = sliceOfLine[0]
	// uri
	uri = sliceOfLine[1]

	// headers
	for {
		headerLine, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("error when reading request header: %v", err)
		}

		headerLine = strings.TrimSuffix(headerLine, "\r\n")
		if headerLine == "" {
			break
		}

		headerSection := strings.SplitN(headerLine, ":", 2)
		if len(headerSection) == 2 {
			headers[strings.TrimSpace(headerSection[0])] = strings.TrimSpace(headerSection[1])
		}

	}

	// body
	if contentLength, ok := headers["Content-Length"]; ok {
		length, err := strconv.Atoi(contentLength)
		if err != nil {
			return nil, fmt.Errorf("error invalid Content-Length header %v", err)
		}

		body := make([]byte, length)

		_, err = io.ReadFull(reader, body)
		if err != nil {
			return nil, fmt.Errorf("error invalid body %v", err)
		}

		reqBody = string(body)

	}

	return &Req{
		Method:  method,
		URI:     uri,
		Body:    reqBody,
		Headers: headers,
	}, nil
}

func NewServer() *Server {
	ln, err := net.Listen("tcp", PORT)
	if err != nil {
		log.Fatal(err)
	}

	return &Server{
		Listener: ln,
	}
}

func NewResponse(status int, data string, contentType string) string {
	responseStatus := fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status))
	responseContentType := fmt.Sprintf("Content-Type: %s\r\n", contentType)
	responseContentLength := fmt.Sprintf("Content-Length: %d\r\n", len(data))

	return responseStatus + responseContentType + responseContentLength + "\r\n" + data
}

func NewRouter(conn net.Conn, req *Req) *Router {
	return &Router{
		Conn: conn,
		Req:  req,
	}
}

func (r *Router) Mux() {
	s := NewService(r)
	if r.Req.Method == "GET" {
		if r.Req.URI == "/" {
			s.handleIndexPage()
		}

		if r.Req.URI == "/about" {
			s.handleAboutPage()
		}
	}

	if r.Req.Method == "POST" {
		if r.Req.URI == "/" {
			s.handleInputData()
		}
	}
}

func NewService(r *Router) *Service {
	store := make(map[string]string)

	return &Service{
		Router: r,
		Store:  store,
	}
}

func (s *Service) handleIndexPage() {
	buffer, err := os.ReadFile("static/index.html")
	if err != nil {
		log.Println(err)
	}

	t, err := template.New("index").Parse(string(buffer))
	if err != nil {
		log.Println("Error when parsing template", err)
		return
	}

    log.Println(s.Store)

	data := map[string]interface{}{
		"data": s.Store,
	}

	var templateBuffer bytes.Buffer
	err = t.Execute(&templateBuffer, data)
	if err != nil {
		log.Println("Error when executing template:", err)
		return
	}

	resp := NewResponse(http.StatusOK, templateBuffer.String(), "text/html")
	s.Router.Conn.Write([]byte(resp))

	defer s.Router.Conn.Close()
}

func (s *Service) handleAboutPage() {
	buffer, err := os.ReadFile("static/about.html")
	if err != nil {
		log.Println(err)
	}

	resp := NewResponse(http.StatusOK, string(buffer), "text/html")
	s.Router.Conn.Write([]byte(resp))
	s.Router.Conn.Close()
}

type Data struct {
	Input string `json:"input"`
}

func (s *Service) handleInputData() {
	log.Println(s.Router.Req.Body)

	inputJson := &Data{}

	err := json.Unmarshal([]byte(s.Router.Req.Body), inputJson)
	if err != nil {
		log.Println("error when unmarshaling request", err)
		return
	}

	log.Println(inputJson.Input)
	s.Store["data"] = inputJson.Input

	resp := NewResponse(http.StatusOK, "success", "application/json")
	s.Router.Conn.Write([]byte(resp))
	s.Router.Conn.Close()
}
