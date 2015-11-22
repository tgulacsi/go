/*
   Copyright 2015 Tamás Gulácsi

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package main of dpdump dumps Camlistore's diskpacked storage.
package main

import (
	"flag"
	"fmt"
	"log"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/blobserver/diskpacked"
	"camlistore.org/pkg/context"
)

func main() {
	flag.Parse()
	stor, err := diskpacked.New(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	stream := stor.(blobserver.BlobStreamer)
	contToken := ""
	for {
		blobs := make(chan blobserver.BlobAndToken, 100)
		go func(dest <-chan blobserver.BlobAndToken) {
			for bt := range dest {
				contToken = bt.Token
				fmt.Printf("blob=%s", bt.Blob)
			}
		}(blobs)
		if err := stream.StreamBlobs(context.New(), blobs, contToken); err != nil {
			log.Printf("ERROR: %v", err)
		}
	}
}
