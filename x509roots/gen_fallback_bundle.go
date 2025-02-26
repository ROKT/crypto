// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build generate

//go:generate go run gen_fallback_bundle.go

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/pem"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"sort"
	"time"

	"golang.org/x/crypto/x509roots/nss"
)

const tmpl = `// Code generated by gen_fallback_bundle.go; DO NOT EDIT.

package fallback

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

type unparsedCertificate struct {
	cn         string
	sha256Hash string
	pem        string

	// possible constraints
	distrustAfter string
}

type parsedCertificate struct {
	cert        *x509.Certificate
	constraints []func([]*x509.Certificate) error
}

func mustParse(unparsedCerts []unparsedCertificate) []parsedCertificate {
	var b []parsedCertificate
	for _, unparsed := range unparsedCerts {
		block, rest := pem.Decode([]byte(unparsed.pem))
		if block == nil {
			panic(fmt.Sprintf("unexpected nil PEM block for %q", unparsed.cn))
		}
		if len(rest) != 0 {
			panic(fmt.Sprintf("unexpected trailing data in PEM for %q", unparsed.cn))
		}
		if block.Type != "CERTIFICATE" {
			panic(fmt.Sprintf("unexpected PEM block type for %q: %s", unparsed.cn, block.Type))
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			panic(err)
		}
		parsed := parsedCertificate{cert: cert}
		// parse possible constraints, this should check all fields of unparsedCertificate.
		if unparsed.distrustAfter != "" {
			distrustAfter, err := time.Parse(time.RFC3339, unparsed.distrustAfter)
			if err != nil {
				panic(fmt.Sprintf("failed to parse distrustAfter %q: %s", unparsed.distrustAfter, err))
			}
			parsed.constraints = append(parsed.constraints, func(chain []*x509.Certificate) error {
				for _, c := range chain {
					if c.NotBefore.After(distrustAfter) {
						return fmt.Errorf("certificate issued after distrust-after date %q", distrustAfter)
					}
				}
				return nil
			})
		}
		b = append(b, parsed)
	}
	return b
}

var parsedCertificates = mustParse(unparsedCertificates)

var unparsedCertificates = []unparsedCertificate{
`

var (
	certDataURL  = flag.String("certdata-url", "https://hg.mozilla.org/mozilla-central/raw-file/tip/security/nss/lib/ckfw/builtins/certdata.txt", "URL to the raw certdata.txt file to parse (certdata-path overrides this, if provided)")
	certDataPath = flag.String("certdata-path", "", "Path to the NSS certdata.txt file to parse (this overrides certdata-url, if provided)")
	output       = flag.String("output", "fallback/bundle.go", "Path to file to write output to")
)

func main() {
	flag.Parse()

	var certdata io.Reader

	if *certDataPath != "" {
		f, err := os.Open(*certDataPath)
		if err != nil {
			log.Fatalf("unable to open %q: %s", *certDataPath, err)
		}
		defer f.Close()
		certdata = f
	} else {
		resp, err := http.Get(*certDataURL)
		if err != nil {
			log.Fatalf("failed to request %q: %s", *certDataURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
			log.Fatalf("got non-200 OK status code: %v body: %q", resp.Status, body)
		} else if ct, want := resp.Header.Get("Content-Type"), `text/plain; charset="UTF-8"`; ct != want {
			if mediaType, _, err := mime.ParseMediaType(ct); err != nil {
				log.Fatalf("bad Content-Type header %q: %v", ct, err)
			} else if mediaType != "text/plain" {
				log.Fatalf("got media type %q, want %q", mediaType, "text/plain")
			}
		}
		certdata = resp.Body
	}

	certs, err := nss.Parse(certdata)
	if err != nil {
		log.Fatalf("failed to parse %q: %s", *certDataPath, err)
	}

	if len(certs) == 0 {
		log.Fatal("certdata.txt appears to contain zero roots")
	}

	sort.Slice(certs, func(i, j int) bool {
		// Sort based on the stringified subject (which may not be unique), and
		// break any ties by just sorting on the raw DER (which will be unique,
		// but is expensive). This should produce a stable sorting, which should
		// be mostly readable by a human looking for a specific root or set of
		// roots.
		subjI, subjJ := certs[i].X509.Subject.String(), certs[j].X509.Subject.String()
		if subjI == subjJ {
			return string(certs[i].X509.Raw) < string(certs[j].X509.Raw)
		}
		return subjI < subjJ
	})

	b := new(bytes.Buffer)
	b.WriteString(tmpl)
	for _, c := range certs {
		var constraints []string
		var skip bool
		for _, constraint := range c.Constraints {
			switch t := constraint.(type) {
			case nss.DistrustAfter:
				constraints = append(constraints, fmt.Sprintf("distrustAfter: \"%s\",", time.Time(t).Format(time.RFC3339)))
			default:
				// If we encounter any constraints we don't support, skip the certificate.
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		fmt.Fprintf(b, "{\ncn: %q,\nsha256Hash: \"%x\",\npem: `%s`,\n",
			c.X509.Subject.String(),
			sha256.Sum256(c.X509.Raw),
			string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.X509.Raw})),
		)
		for _, constraint := range constraints {
			fmt.Fprintln(b, constraint)
		}
		fmt.Fprintln(b, "},")
	}
	fmt.Fprintln(b, "}")

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatalf("failed to format source: %s", err)
	}

	if err := os.WriteFile(*output, formatted, 0644); err != nil {
		log.Fatalf("failed to write to %q: %s", *output, err)
	}
}
