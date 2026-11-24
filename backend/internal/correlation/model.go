package correlation

import "time"

const (
	HasSubdomain        = "HAS_SUBDOMAIN"
	ResolvesTo          = "RESOLVES_TO"
	HasDNSRecord        = "HAS_DNS_RECORD"
	UsesTechnology      = "USES_TECHNOLOGY"
	PresentsCertificate = "PRESENTS_CERTIFICATE"
	HasEndpoint         = "HAS_ENDPOINT"
	HostsService        = "HOSTS_SERVICE"
	RedirectsTo         = "REDIRECTS_TO"
	ObservedIn          = "OBSERVED_IN"
)

var RelationshipTypes = map[string]bool{
	HasSubdomain: true, ResolvesTo: true, HasDNSRecord: true, UsesTechnology: true,
	PresentsCertificate: true, HasEndpoint: true, HostsService: true, RedirectsTo: true, ObservedIn: true,
}

type Relationship struct {
	ID                  string    `json:"id"`
	TargetID            string    `json:"targetId"`
	SourceEntityID      string    `json:"sourceEntityId"`
	DestinationEntityID string    `json:"destinationEntityId"`
	RelationshipType    string    `json:"relationshipType"`
	Confidence          float64   `json:"confidence"`
	FirstSeenAt         time.Time `json:"firstSeenAt"`
	LastSeenAt          time.Time `json:"lastSeenAt"`
	Status              string    `json:"status"`
	AssertionKind       string    `json:"assertionKind"`
}

type Graph struct {
	Nodes []Node         `json:"nodes"`
	Edges []Relationship `json:"edges"`
}

type Node struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	DisplayValue string `json:"displayValue"`
}
