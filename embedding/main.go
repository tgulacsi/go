package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/clipperhouse/uax29/phrases"
	"github.com/clipperhouse/uax29/sentences"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/embeddings/voyageai"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	flagMinWords := flag.Int("min-words", 4, "number of minimum words per sentence")
	flagBatchSize := flag.Int("batch-size", 512, "batch size")
	flagModel := flag.String("model", "voyage-multilingual-2", "model to use")
	flagProvider := flag.String("provider", "voyageai", "provider")
	flagAPIKey := flag.String("api-key", os.Getenv("API_KEY"), "API key")
	if *flagAPIKey == "" {
		*flagAPIKey = `$(gopass show websites/voyageai.com/tamas+voyageai@gulacsi.eu | awk '/first:/ {print $2})'`
	}
	flag.Parse()
	if strings.HasPrefix(*flagAPIKey, "$(") && strings.HasSuffix(*flagAPIKey, ")") {
		shortCtx, shortCancel := context.WithTimeout(ctx, 3*time.Second)
		b, err := exec.CommandContext(shortCtx, "bash", "-c", (*flagAPIKey)[2:len(*flagAPIKey)-1]).Output()
		shortCancel()
		if err != nil {
			return err
		}
		*flagAPIKey = string(b)
	}

	var err error
	var embedder embeddings.Embedder
	switch strings.ToLower(*flagProvider) {
	case "voyageai", "voyage":
		embedder, err = voyageai.NewVoyageAI(
			voyageai.WithBatchSize(*flagBatchSize),
			voyageai.WithModel(*flagModel),
			voyageai.WithToken(*flagAPIKey),
		)
	default:
		err = fmt.Errorf("unknown provider: %q", *flagProvider)
	}
	if err != nil {
		return err
	}

	var buf strings.Builder
	var ss []string
	scanner := sentences.NewScanner(os.Stdin)
	var rem []string
	for scanner.Scan() {
		log.Println("sentence:", scanner.Text())
		if len(bytes.TrimSpace(scanner.Bytes())) <= *flagBatchSize {
			if s := strings.TrimSpace(scanner.Text()); s != "" {
				if len(rem) != 0 {
					s = strings.TrimSpace(strings.Join(append(rem, s), " "))
					rem = rem[:0]
				}
				if len(s) <= 2 || strings.IndexByte(s, ' ') < 0 {
					rem = append(rem, s)
				} else if strings.Count(s, " ") >= *flagMinWords {
					ss = append(ss, s)
				}
			}
			continue
		}
		buf.Reset()
		for _, s := range rem {
			buf.WriteString(s)
		}
		rem = rem[:0]
		segments := phrases.NewSegmenter(scanner.Bytes())
		for segments.Next() {
			buf.Write(segments.Bytes())
			if buf.Len() >= *flagBatchSize {
				ss = append(ss, strings.TrimSpace(buf.String()))
				buf.Reset()
			}
		}
		if buf.Len() != 0 {
			if s := strings.TrimSpace(buf.String()); len(s) > 2 {
				ss = append(ss, s)
			}
		}
	}
	log.Printf("sentences[%d]: %q", len(ss), ss)

	B, err := embedder.EmbedDocuments(ctx, ss)
	if err != nil {
		return err
	}
	log.Println("B:", len(B))

	return nil
}
