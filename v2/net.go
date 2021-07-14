package httpserver

import (
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type NetCfg interface {
	GetNetwork() *net.IPNet
	GetInterface() *net.Interface
	GetIP() net.IP
}

type NetworkConfig struct {
	Network      string `json:"network"   arg:"network"`
	Interface    string `json:"interface" arg:"interface"`
	IP           string `json:"ip"        arg:"ip"`
	networkObj   *net.IPNet
	interfaceObj *net.Interface
	ipObj        net.IP
}

func ParseIPNet(netstr string) *net.IPNet {
	dflt := &net.IPNet{
		IP: net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}
	parts := strings.Split(netstr, "/")
	if len(parts) != 2 {
		return dflt
	}
	ip := net.ParseIP(parts[0])
	if ip == nil {
		return dflt
	}
	maskbits, err := strconv.Atoi(parts[1])
	if err != nil {
		return dflt
	}
	mask := net.CIDRMask(maskbits, 32)
	return &net.IPNet{
		IP: ip,
		Mask: mask,
	}
}

func (cfg *NetworkConfig) GetNetwork() *net.IPNet {
	if cfg.networkObj != nil {
		return cfg.networkObj
	}
	if cfg.Network != "" {
		cfg.networkObj = ParseIPNet(cfg.Network)
		return cfg.networkObj
	}
	if cfg.IP != "" {
		ip := net.ParseIP(cfg.IP)
		cfg.networkObj = &net.IPNet{
			IP: ip,
			Mask: ip.DefaultMask(),
		}
		cfg.Network = cfg.networkObj.String()
		return cfg.networkObj
	}
	if cfg.Interface != "" {
		iface := cfg.GetInterface()
		ips, err := interfaceIps(iface)
		if err == nil {
			for _, ip := range ips {
				cfg.networkObj = &net.IPNet{
					IP: ip,
					Mask: ip.DefaultMask(),
				}
				cfg.Network = cfg.networkObj.String()
				return cfg.networkObj
			}
		}
	}
	cfg.networkObj = &net.IPNet{
		IP: net.ParseIP("0.0.0.0"),
		Mask: net.CIDRMask(0, 32),
	}
	cfg.Network = cfg.networkObj.String()
	return cfg.networkObj
}

func (cfg *NetworkConfig) GetInterface() *net.Interface {
	if cfg.interfaceObj != nil {
		return cfg.interfaceObj
	}
	if cfg.Interface != "" {
		iface, err := net.InterfaceByName(cfg.Interface)
		if err != nil {
			log.Printf("error looking up interface %s: %s", cfg.Interface, err)
			return nil
		}
		cfg.interfaceObj = iface
		return cfg.interfaceObj
	}
	var n *net.IPNet
	if cfg.IP != "" {
		n = &net.IPNet{
			IP: cfg.GetIP(),
			Mask: net.CIDRMask(32, 32),
		}
	} else if cfg.Network != "" {
		n = cfg.GetNetwork()
	} else {
		n = &net.IPNet{
			IP: net.ParseIP("0.0.0.0"),
			Mask: net.CIDRMask(0, 32),
		}
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("error looking up interfaces: %s", err)
		return nil
	}
	for _, iface := range ifaces {
		ips, err := interfaceIps(&iface)
		if err != nil {
			log.Printf("error getting network addresses for interface %s: %s", iface.Name, err)
			continue
		}
		for _, ip := range ips {
			if n.Contains(ip) {
				cfg.interfaceObj = &iface
				cfg.Interface = iface.Name
				return cfg.interfaceObj
			}
		}
	}
	log.Printf("no interface is bound on network %s", n.String())
	return nil
}

func (cfg *NetworkConfig) GetIP() net.IP {
	if cfg.ipObj != nil {
		return cfg.ipObj
	}
	if cfg.IP != "" {
		cfg.ipObj = net.ParseIP(cfg.IP)
		return cfg.ipObj
	}
	var ifaces []net.Interface
	if cfg.Interface != "" {
		iface := cfg.GetInterface()
		if iface != nil {
			ifaces = []net.Interface{*iface}
		} else {
			return nil
		}
	} else {
		var err error
		ifaces, err = net.Interfaces()
		if err != nil {
			log.Printf("error getting network interfaces: %s", err)
			return nil
		}
	}
	var n *net.IPNet
	if cfg.Network != "" {
		n = cfg.GetNetwork()
	} else {
		n = &net.IPNet{
			IP: net.IP{0,0,0,0},
			Mask: net.CIDRMask(0, 32),
		}
	}
	for _, iface := range ifaces {
		ips, err := interfaceIps(&iface)
		if err != nil {
			log.Printf("error getting addresses for interface %s: %s", iface.Name, err)
			return nil
		}
		for _, ip := range ips {
			if n.Contains(ip) {
				cfg.ipObj = ip
				cfg.IP = cfg.ipObj.String()
				return cfg.ipObj
			}
		}
	}
	if len(ifaces) == 1 {
		log.Printf("no addresses found for interface %s on network %s", ifaces[0].Name, n.String())
	} else {
		log.Printf("no addresses found on network %s", n.String())
	}
	return nil
}

func (cfg *NetworkConfig) Init() error {
	if cfg.Network != "" {
		cfg.GetNetwork()
	}
	if cfg.Interface != "" {
		cfg.GetInterface()
	}
	if cfg.IP != "" {
		cfg.GetIP()
	}
	if cfg.Network == "" {
		cfg.GetNetwork()
	}
	if cfg.Interface == "" {
		cfg.GetInterface()
	}
	if cfg.IP == "" {
		cfg.GetIP()
	}
	return nil
}

func interfaceIps(iface *net.Interface) ([]net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, errors.Wrap(err, "can't get addresses for interface " + iface.Name)
	}
	ips := []net.IP{}
	for _, addr := range addrs {
		var ip net.IP
		switch addrt := addr.(type) {
		case *net.IPAddr:
			ip = addrt.IP
		case *net.IPNet:
			ip = addrt.IP
		default:
			continue
		}
		if ip != nil && ip.To4() != nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

