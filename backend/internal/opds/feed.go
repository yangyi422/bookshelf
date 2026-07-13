package opds

import "encoding/xml"

const (
	atomNS  = "http://www.w3.org/2005/Atom"
	opdsNS  = "http://opds-spec.org/2010/catalog"
	dcNS    = "http://purl.org/dc/terms/"
	navType = "application/atom+xml;profile=opds-catalog;kind=navigation"
	acqType = "application/atom+xml;profile=opds-catalog;kind=acquisition"
)

type Link struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}
type Person struct {
	Name string `xml:"name"`
}
type Category struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr,omitempty"`
}
type Content struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}
type Entry struct {
	ID         string     `xml:"id"`
	Title      string     `xml:"title"`
	Updated    string     `xml:"updated"`
	Authors    []Person   `xml:"author,omitempty"`
	Summary    string     `xml:"summary,omitempty"`
	Language   string     `xml:"dc:language,omitempty"`
	Publisher  string     `xml:"dc:publisher,omitempty"`
	Identifier string     `xml:"dc:identifier,omitempty"`
	Categories []Category `xml:"category,omitempty"`
	Content    *Content   `xml:"content,omitempty"`
	Links      []Link     `xml:"link"`
}
type Feed struct {
	XMLName   xml.Name `xml:"feed"`
	XMLNS     string   `xml:"xmlns,attr"`
	XMLNSOPDS string   `xml:"xmlns:opds,attr"`
	XMLNSDC   string   `xml:"xmlns:dc,attr"`
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Updated   string   `xml:"updated"`
	Links     []Link   `xml:"link"`
	Entries   []Entry  `xml:"entry"`
}

func newFeed(id, title, updated string) Feed {
	return Feed{XMLNS: atomNS, XMLNSOPDS: opdsNS, XMLNSDC: dcNS, ID: id, Title: title, Updated: updated}
}

type OpenSearch struct {
	XMLName       xml.Name      `xml:"OpenSearchDescription"`
	XMLNS         string        `xml:"xmlns,attr"`
	ShortName     string        `xml:"ShortName"`
	Description   string        `xml:"Description"`
	InputEncoding string        `xml:"InputEncoding"`
	URL           OpenSearchURL `xml:"Url"`
}
type OpenSearchURL struct {
	Type     string `xml:"type,attr"`
	Template string `xml:"template,attr"`
}
