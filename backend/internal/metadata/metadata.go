package metadata

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const maxEntry = 20 << 20
const maxTotal = 100 << 20

type Result struct {
	Title                                        string
	Authors                                      []string
	Language, Publisher, Identifier, Description string
	PageCount                                    int
	Cover                                        []byte
	CoverMIME, CoverExt                          string
}

func Parse(filePath, format string) (Result, error) {
	switch format {
	case "epub":
		return parseEPUB(filePath)
	case "pdf":
		return parsePDF(filePath)
	default:
		return Result{}, nil
	}
}

type container struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}
type packageDoc struct {
	Metadata struct {
		Title       string   `xml:"title"`
		Creators    []string `xml:"creator"`
		Language    string   `xml:"language"`
		Publisher   string   `xml:"publisher"`
		Identifiers []string `xml:"identifier"`
		Description string   `xml:"description"`
		Metas       []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
}

func parseEPUB(name string) (Result, error) {
	zr, err := zip.OpenReader(name)
	if err != nil {
		return Result{}, fmt.Errorf("open EPUB archive: %w", err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range zr.File {
		clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasPrefix(clean, "../") || clean == ".." || path.IsAbs(clean) {
			return Result{}, fmt.Errorf("unsafe EPUB path %q", f.Name)
		}
		if f.UncompressedSize64 > maxEntry {
			return Result{}, fmt.Errorf("EPUB entry %q exceeds size limit", f.Name)
		}
		total += f.UncompressedSize64
		if total > maxTotal {
			return Result{}, errors.New("EPUB uncompressed content exceeds size limit")
		}
		files[clean] = f
	}
	containerBytes, err := readZip(files["META-INF/container.xml"])
	if err != nil {
		return Result{}, fmt.Errorf("read EPUB container.xml: %w", err)
	}
	var c container
	if err := xml.Unmarshal(containerBytes, &c); err != nil || len(c.Rootfiles) == 0 {
		return Result{}, errors.New("EPUB container.xml has no valid rootfile")
	}
	opfPath := path.Clean(c.Rootfiles[0].FullPath)
	opfBytes, err := readZip(files[opfPath])
	if err != nil {
		return Result{}, fmt.Errorf("read EPUB package: %w", err)
	}
	var pkg packageDoc
	if err := xml.Unmarshal(opfBytes, &pkg); err != nil {
		return Result{}, fmt.Errorf("parse EPUB package: %w", err)
	}
	r := Result{Title: strings.TrimSpace(pkg.Metadata.Title), Language: strings.TrimSpace(pkg.Metadata.Language), Publisher: strings.TrimSpace(pkg.Metadata.Publisher), Description: strings.TrimSpace(pkg.Metadata.Description)}
	for _, a := range pkg.Metadata.Creators {
		if v := strings.TrimSpace(a); v != "" {
			r.Authors = append(r.Authors, v)
		}
	}
	if len(pkg.Metadata.Identifiers) > 0 {
		r.Identifier = strings.TrimSpace(pkg.Metadata.Identifiers[0])
	}
	coverID := ""
	for _, m := range pkg.Metadata.Metas {
		if strings.EqualFold(m.Name, "cover") {
			coverID = m.Content
		}
	}
	for _, item := range pkg.Manifest {
		if item.ID == coverID || strings.Contains(item.Properties, "cover-image") {
			coverPath := path.Clean(path.Join(path.Dir(opfPath), item.Href))
			data, e := readZip(files[coverPath])
			if e != nil {
				continue
			}
			ext, mime := DetectCover(data, item.MediaType)
			if ext != "" {
				r.Cover = data
				r.CoverExt = ext
				r.CoverMIME = mime
			}
			break
		}
	}
	return r, nil
}
func readZip(f *zip.File) ([]byte, error) {
	if f == nil {
		return nil, os.ErrNotExist
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxEntry+1))
}
func DetectCover(b []byte, hint string) (string, string) {
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return ".jpg", "image/jpeg"
	}
	if len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return ".png", "image/png"
	}
	if hint == "image/webp" && len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return ".webp", "image/webp"
	}
	return "", ""
}

var pdfTitle = regexp.MustCompile(`/Title\s*\(([^)]*)\)`)
var pdfAuthor = regexp.MustCompile(`/Author\s*\(([^)]*)\)`)
var pdfPage = regexp.MustCompile(`/Type\s*/Page(?:\s|/|>>)`)

func parsePDF(name string) (Result, error) {
	f, err := os.Open(name)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16<<20))
	if err != nil {
		return Result{}, err
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return Result{}, errors.New("invalid PDF header")
	}
	r := Result{PageCount: len(pdfPage.FindAll(data, -1))}
	if m := pdfTitle.FindSubmatch(data); len(m) > 1 {
		r.Title = pdfString(m[1])
	}
	if m := pdfAuthor.FindSubmatch(data); len(m) > 1 {
		if a := pdfString(m[1]); a != "" {
			r.Authors = []string{a}
		}
	}
	return r, nil
}
func pdfString(b []byte) string {
	s := string(b)
	s = strings.ReplaceAll(s, `\(`, "(")
	s = strings.ReplaceAll(s, `\)`, ")")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return strings.TrimSpace(s)
}
func ISBN(identifier string) string {
	v := strings.TrimSpace(identifier)
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "urn:isbn:") {
		v = v[9:]
	}
	digits := strings.NewReplacer("-", "", " ", "").Replace(v)
	if len(digits) == 10 || len(digits) == 13 {
		if _, err := strconv.ParseUint(strings.TrimSuffix(digits, "X"), 10, 64); err == nil {
			return v
		}
	}
	return ""
}
