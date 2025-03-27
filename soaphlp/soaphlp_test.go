// Copyright 2025 Tamas Gulacsi. All rights reserved.

package soaphlp_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/tgulacsi/go/soaphlp"
)

func TestFindBody(t *testing.T) {
	const reqText = `<?xml version="1.0" encoding="utf-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:head="http://services.alfa.hu/Header/" >
  <soapenv:Header>
    <head:IMSSOAPHeader>
         <ConnectionStyle>SYNCHRONOUS</ConnectionStyle>
         <SystemId>153</SystemId>
         <ActualUserId>xx</ActualUserId>
         <RequestDateTime>2025-03-27T15:10:00.000</RequestDateTime>
         <TargetServiceID>136</TargetServiceID>
         <TransactionID>tran-sact-ion-id</TransactionID>
    </head:IMSSOAPHeader>
 </soapenv:Header>
<soapenv:Body>
<UploadRequest>
  <Tartomany>splprn</Tartomany>
  <FileName>1_2.pdf</FileName>
  <Data>DATA</Data>
</UploadRequest>
</soapenv:Body>
</soapenv:Envelope>`
	var hdr struct {
		Header string `xml:",innerxml"`
	}
	dec, err := soaphlp.FindBody(&hdr, strings.NewReader(reqText))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("header=%q", hdr)
	if strings.TrimSpace(hdr.Header) == "" || !strings.Contains(hdr.Header, "tran-sact-ion-id") {
		t.Error("empty header")
	}
	var upReq struct {
		XMLName                   xml.Name `xml:"UploadRequest"`
		Tartomany, FileName, Data string
	}
	if err = dec.Decode(&upReq); err != nil {
		t.Fatal(err)
	}
	if !(upReq.Tartomany == "splprn" && upReq.FileName == "1_2.pdf") {
		t.Errorf("decoded %#v", upReq)
	}

}
