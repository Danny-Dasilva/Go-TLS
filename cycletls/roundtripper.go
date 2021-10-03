package cycletls

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"log"

	// "log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"

	utls "github.com/refraction-networking/utls"
)

var errProtocolNegotiated = errors.New("protocol negotiated")

type errExtensionNotExist string

func (err errExtensionNotExist) Error() string {
	return fmt.Sprintf("Extension does not exist: %s\n", err)
}

type roundTripper struct {
	sync.Mutex
	// fix typing
	JA3       string
	UserAgent string

	Cookies           []Cookie
	cachedConnections map[string]net.Conn
	cachedTransports  map[string]http.RoundTripper

	dialer proxy.ContextDialer
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// This is dumb but whatever
	for _, properties := range rt.Cookies {
		req.AddCookie(&http.Cookie{Name: properties.Name,
			Value:      properties.Value,
			Path:       properties.Path,
			Domain:     properties.Domain,
			Expires:    properties.JSONExpires.Time, //TODO: scuffed af
			RawExpires: properties.RawExpires,
			MaxAge:     properties.MaxAge,
			HttpOnly:   properties.HTTPOnly,
			Secure:     properties.Secure,
			SameSite:   properties.SameSite,
			Raw:        properties.Raw,
			Unparsed:   properties.Unparsed,
		})
		fmt.Println(properties.Raw)
	}
	req.Header.Set("User-Agent", rt.UserAgent)
	addr := rt.getDialTLSAddr(req)
	if _, ok := rt.cachedTransports[addr]; !ok {
		if err := rt.getTransport(req, addr); err != nil {
			return nil, err
		}
	}
	return rt.cachedTransports[addr].RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request, addr string) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		rt.cachedTransports[addr] = &http.Transport{DialContext: rt.dialer.DialContext, DisableKeepAlives: true}
		return nil
	case "https":
	default:
		return fmt.Errorf("invalid URL scheme: [%v]", req.URL.Scheme)
	}

	_, err := rt.dialTLS(context.Background(), "tcp", addr)
	switch err {
	case errProtocolNegotiated:
	case nil:
		// Should never happen.
		panic("dialTLS returned no error when determining cachedTransports")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	rt.Lock()
	defer rt.Unlock()

	// If we have the connection from when we determined the HTTPS
	// cachedTransports to use, return that.
	if conn := rt.cachedConnections[addr]; conn != nil {
		delete(rt.cachedConnections, addr)
		return conn, nil
	}
	rawConn, err := rt.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}
	//////////////////

	spec, err := stringToSpec(rt.JA3)
	if err != nil {
		return nil, err
	}

	// spec = &utls.ClientHelloSpec{
	// 	CipherSuites: []uint16{
	// 		utls.GREASE_PLACEHOLDER,
	// 		utls.TLS_AES_128_GCM_SHA256,
	// 		utls.TLS_AES_256_GCM_SHA384,
	// 		utls.TLS_CHACHA20_POLY1305_SHA256,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	// 		utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	// 		utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	// 		utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	// 		utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	// 		utls.TLS_RSA_WITH_AES_128_CBC_SHA,
	// 		utls.TLS_RSA_WITH_AES_256_CBC_SHA,
	// 	},
	// 	CompressionMethods: []byte{
	// 		0x00, // compressionNone
	// 	},
	// 	Extensions: []utls.TLSExtension{
	// 		&utls.UtlsGREASEExtension{},
	// 		&utls.SNIExtension{},
	// 		&utls.UtlsExtendedMasterSecretExtension{},
	// 		&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
	// 		&utls.SupportedCurvesExtension{[]utls.CurveID{
	// 			// utls.CurveID(utls.GREASE_PLACEHOLDER),
	// 			utls.X25519,
	// 			utls.CurveP256,
	// 			utls.CurveP384,
	// 		}},
	// 		&utls.SupportedPointsExtension{SupportedPoints: []byte{
	// 			0x00, // pointFormatUncompressed
	// 		}},
	// 		&utls.SessionTicketExtension{},
	// 		&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
	// 		&utls.StatusRequestExtension{},
	// 		&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
	// 			utls.ECDSAWithP256AndSHA256,
	// 			utls.PSSWithSHA256,
	// 			utls.PKCS1WithSHA256,
	// 			utls.ECDSAWithP384AndSHA384,
	// 			utls.PSSWithSHA384,
	// 			utls.PKCS1WithSHA384,
	// 			utls.PSSWithSHA512,
	// 			utls.PKCS1WithSHA512,
	// 		}},
	// 		&utls.SCTExtension{},
	// 		&utls.KeyShareExtension{[]utls.KeyShare{
	// 			{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
	// 			{Group: utls.X25519},
	// 		}},
	// 		&utls.PSKKeyExchangeModesExtension{[]uint8{
	// 			utls.PskModeDHE,
	// 		}},
	// 		&utls.SupportedVersionsExtension{[]uint16{
	// 			utls.GREASE_PLACEHOLDER,
	// 			utls.VersionTLS13,
	// 			utls.VersionTLS12,
	// 			utls.VersionTLS11,
	// 			utls.VersionTLS10,
	// 		}},
	// 		&utls.FakeCertCompressionAlgsExtension{[]utls.CertCompressionAlgo{
	// 			utls.CertCompressionBrotli,
	// 		}},
	// 		// &utls.UtlsGREASEExtension{},
	// 		&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
	// 	},
	// }
	// spec = &utls.ClientHelloSpec{
	// 	TLSVersMin: utls.VersionTLS10,
	// 	TLSVersMax: utls.VersionTLS13,
	// 	CipherSuites: []uint16{
	// 		utls.TLS_AES_128_GCM_SHA256,
	// 		utls.TLS_CHACHA20_POLY1305_SHA256,
	// 		utls.TLS_AES_256_GCM_SHA384,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	// 		utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	// 		utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	// 		utls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	// 		utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	// 		utls.FAKE_TLS_DHE_RSA_WITH_AES_128_CBC_SHA,
	// 		utls.FAKE_TLS_DHE_RSA_WITH_AES_256_CBC_SHA,
	// 		utls.TLS_RSA_WITH_AES_128_CBC_SHA,
	// 		utls.TLS_RSA_WITH_AES_256_CBC_SHA,
	// 		utls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	// 	},
	// 	CompressionMethods: []byte{
	// 		0x00,
	// 	},
	// 	Extensions: []utls.TLSExtension{
	// 		&utls.SNIExtension{},
	// 		&utls.UtlsExtendedMasterSecretExtension{},
	// 		&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
	// 		&utls.SupportedCurvesExtension{[]utls.CurveID{
	// 			utls.X25519,
	// 			utls.CurveP256,
	// 			utls.CurveP384,
	// 			utls.CurveP521,
	// 			utls.CurveID(utls.FakeFFDHE2048),
	// 			utls.CurveID(utls.FakeFFDHE3072),
	// 		}},
	// 		&utls.SupportedPointsExtension{SupportedPoints: []byte{
	// 			0x00,
	// 		}},
	// 		&utls.SessionTicketExtension{},
	// 		&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
	// 		&utls.StatusRequestExtension{},
	// 		&utls.KeyShareExtension{[]utls.KeyShare{
	// 			{Group: utls.X25519},
	// 			{Group: utls.CurveP256},
	// 		}},
	// 		&utls.SupportedVersionsExtension{[]uint16{
	// 			utls.VersionTLS13,
	// 			utls.VersionTLS12,
	// 			utls.VersionTLS11,
	// 			utls.VersionTLS10}},
	// 		&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
	// 			utls.ECDSAWithP256AndSHA256,
	// 			utls.ECDSAWithP384AndSHA384,
	// 			utls.ECDSAWithP521AndSHA512,
	// 			utls.PSSWithSHA256,
	// 			utls.PSSWithSHA384,
	// 			utls.PSSWithSHA512,
	// 			utls.PKCS1WithSHA256,
	// 			utls.PKCS1WithSHA384,
	// 			utls.PKCS1WithSHA512,
	// 			utls.ECDSAWithSHA1,
	// 			utls.PKCS1WithSHA1,
	// 		}},
	// 		&utls.PSKKeyExchangeModesExtension{[]uint8{utls.PskModeDHE}},
	// 		&utls.FakeRecordSizeLimitExtension{0x4001},
	// 		&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
	// 	}}

	conn := utls.UClient(rawConn, &utls.Config{ServerName: host}, // MinVersion:         tls.VersionTLS10,
		// MaxVersion:         tls.VersionTLS12, // Default is TLS13
		utls.HelloCustom)
	if err := conn.ApplyPreset(spec); err != nil {
		return nil, err
	}

	if err = conn.Handshake(); err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("uTlsConn.Handshake() error: %+v", err)
	}

	//////////
	if rt.cachedTransports[addr] != nil {
		return conn, nil
	}

	// No http.Transport constructed yet, create one based on the results
	// of ALPN.
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		// The remote peer is speaking HTTP 2 + TLS.
		rt.cachedTransports[addr] = &http2.Transport{DialTLS: rt.dialTLSHTTP2}
	default:
		// Assume the remote peer is speaking HTTP 1.x + TLS.
		rt.cachedTransports[addr] = &http.Transport{DialTLSContext: rt.dialTLS}

	}

	// Stash the connection just established for use servicing the
	// actual request (should be near-immediate).
	rt.cachedConnections[addr] = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, _ *tls.Config) (net.Conn, error) {
	return rt.dialTLS(context.Background(), network, addr)
}

func (rt *roundTripper) getDialTLSAddr(req *http.Request) string {
	host, port, err := net.SplitHostPort(req.URL.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(req.URL.Host, "443") // we can assume port is 443 at this point
}

func newRoundTripper(browser browser, dialer ...proxy.ContextDialer) http.RoundTripper {
	if len(dialer) > 0 {

		return &roundTripper{
			dialer: dialer[0],

			JA3:               browser.JA3,
			UserAgent:         browser.UserAgent,
			Cookies:           browser.Cookies,
			cachedTransports:  make(map[string]http.RoundTripper),
			cachedConnections: make(map[string]net.Conn),
		}
	}

	return &roundTripper{
		dialer: proxy.Direct,

		JA3:               browser.JA3,
		UserAgent:         browser.UserAgent,
		Cookies:           browser.Cookies,
		cachedTransports:  make(map[string]http.RoundTripper),
		cachedConnections: make(map[string]net.Conn),
	}
}

///////////////////////// test code
// stringToSpec creates a ClientHelloSpec based on a JA3 string
func stringToSpec(ja3 string) (*utls.ClientHelloSpec, error) {
	extMap := genMap()
	tokens := strings.Split(ja3, ",")

	version := tokens[0]
	ciphers := strings.Split(tokens[1], "-")
	extensions := strings.Split(tokens[2], "-")
	curves := strings.Split(tokens[3], "-")
	if len(curves) == 1 && curves[0] == "" {
		curves = []string{}
	}
	pointFormats := strings.Split(tokens[4], "-")
	if len(pointFormats) == 1 && pointFormats[0] == "" {
		pointFormats = []string{}
	}

	// parse curves
	var targetCurves []utls.CurveID
	targetCurves = append(targetCurves, utls.CurveID(utls.CurveID(utls.GREASE_PLACEHOLDER)))
	//append grease for Chrome browsers
	for _, c := range curves {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		targetCurves = append(targetCurves, utls.CurveID(cid))
	}
	extMap["10"] = &utls.SupportedCurvesExtension{Curves: targetCurves}

	// parse point formats
	var targetPointFormats []byte
	for _, p := range pointFormats {
		pid, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, err
		}
		targetPointFormats = append(targetPointFormats, byte(pid))
	}
	extMap["11"] = &utls.SupportedPointsExtension{SupportedPoints: targetPointFormats}

	// set extension 43
	vid64, err := strconv.ParseUint(version, 10, 16)
	if err != nil {
		return nil, err
	}
	vid := uint16(vid64)
	// extMap["43"] = &utls.SupportedVersionsExtension{
	// 	Versions: []uint16{
	// 		utls.VersionTLS12,
	// 	},
	// }

	// build extenions list
	var exts []utls.TLSExtension
	for _, e := range extensions {
		te, ok := extMap[e]
		log.Println(e)

		if !ok {
			return nil, errExtensionNotExist(e)
		}
		exts = append(exts, te)
	}
	// build SSLVersion
	// vid64, err := strconv.ParseUint(version, 10, 16)
	// if err != nil {
	// 	return nil, err
	// }

	// build CipherSuites
	var suites []uint16
	for _, c := range ciphers {
		cid, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, err
		}
		suites = append(suites, uint16(cid))
	}
	_ = vid
	return &utls.ClientHelloSpec{
		// TLSVersMin:         vid,
		// TLSVersMax:         vid,
		CipherSuites:       suites,
		CompressionMethods: []byte{0},
		Extensions:         exts,
		GetSessionID:       sha256.Sum256,
	}, nil
}

func genMap() (extMap map[string]utls.TLSExtension) {
	log.Println(&utls.UtlsGREASEExtension{},)
	extMap = map[string]utls.TLSExtension{
		"0": &utls.SNIExtension{},
		"5": &utls.StatusRequestExtension{},
		// These are applied later
		// "10": &tls.SupportedCurvesExtension{...}
		// "11": &tls.SupportedPointsExtension{...}
		"13": &utls.SignatureAlgorithmsExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.ECDSAWithP521AndSHA512,
				utls.PSSWithSHA256,
				utls.PSSWithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA256,
				utls.PKCS1WithSHA384,
				utls.PKCS1WithSHA512,
				utls.ECDSAWithSHA1,
				utls.PKCS1WithSHA1,
			},
		},
		"16": &utls.ALPNExtension{
			AlpnProtocols: []string{"h2", "http/1.1"},
		},
		"18": &utls.SCTExtension{},
		"21": &utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
		"22": &utls.GenericExtension{Id: 22}, // encrypt_then_mac
		"23": &utls.UtlsExtendedMasterSecretExtension{},
		"27": &utls.FakeCertCompressionAlgsExtension{},
		"28": &utls.FakeRecordSizeLimitExtension{0x4001},
		"35": &utls.SessionTicketExtension{},
		"34": &utls.GenericExtension{Id: 34},
		"43": &utls.SupportedVersionsExtension{Versions: []uint16{
			// utls.GREASE_PLACEHOLDER,
			utls.VersionTLS13,
			utls.VersionTLS12,
			utls.VersionTLS11,
			utls.VersionTLS10}},
		// "44": &utls.CookieExtension{},
		"45": &utls.PSKKeyExchangeModesExtension{Modes: []uint8{
			utls.PskModeDHE,
		}},
		"49": &utls.GenericExtension{Id: 49}, // post_handshake_auth
		"50": &utls.GenericExtension{Id: 50}, // signature_algorithms_cert
		"51": &utls.KeyShareExtension{KeyShares: []utls.KeyShare{
				{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
				{Group: utls.X25519},
			{Group: utls.CurveP256},
			
			// {Group: utls.CurveP384},
		}},
		"13172": &utls.NPNExtension{},
		"65281": &utls.RenegotiationInfoExtension{
			Renegotiation: utls.RenegotiateOnceAsClient,
		},
	}
	return

}
