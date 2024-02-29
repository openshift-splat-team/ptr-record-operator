package data

type Subnet struct {
	CIDR             int64    `json:"cidr"`
	CidrIPv6         int64    `json:"cidrIPv6"`
	DnsServer        string   `json:"dnsServer"`
	Gateway          string   `json:"gateway"`
	Mask             string   `json:"mask"`
	Network          string   `json:"network"`
	IpAddresses      []string `json:"ipAddresses"`
	Virtualcenter    string   `json:"virtualcenter"`
	Ipv6prefix       string   `json:"ipv6prefix"`
	StartIPv6Address string   `json:"startIPv6Address"`
	StopIPv6Address  string   `json:"stopIPv6Address"`
	LinkLocalIPv6    string   `json:"linkLocalIPv6"`
	VifIpAddress     string   `json:"vifIpAddress"`
	VifIPv6Address   string   `json:"vifIPv6Address"`
	DhcpEndLocation  int      `json:"dhcpEndLocation"`
	Priority         int      `json:"priority"`
}
