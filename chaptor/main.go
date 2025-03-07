package main

/*
TODO
- adding email functionality
*/

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"golang.org/x/net/html"
)

var (
	Version  = "1.2.0"
	CommitId string
)

func main() {
	var urlFile string
	sleepTime := 6 * time.Second
	app := &cli.Command{
		Name:    "chaptor",
		Usage:   "Royal road chapter extraction",
		Version:   fmt.Sprintf("%s (%s)", Version, CommitId),

		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "activates debugging messages",
				Action: func(ctx context.Context, cmd *cli.Command, shouldDebug bool) error {
					if shouldDebug {
						logrus.SetLevel(logrus.DebugLevel)
					}
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "single",
				Usage: "Extract a single",
				Action: func(cCtx context.Context, cmd *cli.Command) error {
					list := cmd.Args().Slice()
					_, content := requestchapter(list[0])
					saveFile(content)
					return nil
				},
			},
			{
				Name:  "many",
				Usage: "complete a task on the list",
				Action: func(cCtx context.Context, cmd *cli.Command) error {
					pagebreak := `<mbp:pagebreak />
`
					var urlList []string = make([]string, 0, 30)
					if urlFile != "" {
						file, err := os.Open(urlFile)
						errorutils.ExitOnFail(err)
						defer errorutils.NotifyClose(file)
						scanner := bufio.NewScanner(file)
						scanner.Split(bufio.ScanLines) // Split on newline characters

						for scanner.Scan() {
							if len(scanner.Text()) == 0 {
								continue
							}
							urlList = append(urlList, scanner.Text())
						}
					} else {
						urlList = cmd.Args().Slice()
					}

					file, err := os.OpenFile(selectFile("chapter"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
					errorutils.WarnOnFail(err, errorutils.WithLineRef("KSLhF2YZIEI"))
					defer errorutils.NotifyClose(file)

					var sb strings.Builder // exists to create the TOC simultaniously

					writer := bufio.NewWriter(file)
					defer writer.Flush()
					defer writer.WriteString(`
</body>
</html>`)
					writer.WriteString(`<!DOCTYPE html>
<html>
<body>
<div class="table-of-contents">
<ul>
`)

					for i, url := range urlList {
						logrus.Debug("url was " + url + "\nworking on No. " + fmt.Sprint(i))

						tocEntry, chapter := requestchapter(url)
						writer.WriteString(tocEntry)
						sb.WriteString(chapter)
						//tracker
						fmt.Print(len(urlList) - i)
						if len(urlList)-i-1 != 0 {
							sb.WriteString(pagebreak)
						}
						time.Sleep(sleepTime)
						//keep to oneline
						fmt.Print("\033[2K\r")
					}
					writer.WriteString(`</ul>
</div>
<mbp:pagebreak />
`)
					writer.WriteString(sb.String())

					logrus.Info(fmt.Sprintf("A total of %d chapters were writen to %s", len(urlList), file.Name()))
					return nil
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "listFile",
						Aliases:     []string{"l"},
						Usage:       "URLs to go through as newline delimited list in `FILE`",
						Destination: &urlFile,
					},
				},
			},
			{
				Name: "remind",
				Action: func(cCtx context.Context, cmd *cli.Command) error {
					printMyJS()
					return nil
				},
			},
			{
				Name: "updateFilter",
				Action: func(cCtx context.Context, cmd *cli.Command) error {
					fmt.Println(generateFilter())
					fmt.Printf("Working with filterID %s.\nRememeber to change the id in filer.go when updating it.\n", filterID)
					return nil
				},
				Hidden: true,
			},
		},
	}

	runningErr := app.Run(context.Background(), os.Args)
	errorutils.ExitOnFail(runningErr)

}

func requestchapter(url string) (tocEntry, chapter string) {
	resp, err := http.Get(url)
	errorutils.ExitOnFail(err)
	defer resp.Body.Close()
	tt, id, chapter := composeChapter(resp)
	tocEntry = fmt.Sprintf(`<li>
<a href="#%s">%s</a>
</li>
`, id, tt)
	return

}

func composeChapter(resp *http.Response) (titleText, id, chapter string) {
	body, err := io.ReadAll(resp.Body)
	errorutils.WarnOnFailf(err, "Error reading response body: %s",
		errorutils.WithExitCode(1), errorutils.WithLineRef("GLJpIaU"))

	// Parse the HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	errorutils.ExitOnFail(err, errorutils.WithMsg("Error parsing HTML"), errorutils.WithLineRef("HzIfWabz"))

	if logrus.StandardLogger().GetLevel() == logrus.DebugLevel {
		saveDoc(doc)
	}

	// Find the target div
	targetDiv := findElement(doc, "div", "chapter-inner chapter-content")
	targetTitle := findElement(doc, "h1", "font-white break-word")

	// Return variables
	var content bytes.Buffer
	var titler bytes.Buffer
	identifier := generateRandomString()

	if targetDiv != nil {
		err := html.Render(&content, targetDiv)
		stashFail := errorutils.HandleFailure(err,
			errorutils.Handler(func() *errorutils.Details {
				f, openingErr := os.Create("~/Downloads/failedExtract.html")
				defer errorutils.NotifyClose(f)
				errorutils.WarnOnFail(openingErr, errorutils.WithLineRef("u19nn0w1G7I"))
				return errorutils.New(resp.Write(f))
			}))
		errorutils.WarnOnFail(stashFail, errorutils.WithLineRef("rnJkMvgQFue"))
		errorutils.ExitOnFail(err, errorutils.WithMsg("response saved as ~/Downloads/failedExtract"))

		if targetTitle != nil {
			targetTitle.Attr = append(targetTitle.Attr, html.Attribute{Key: "id", Val: identifier})
			err := html.Render(&titler, targetTitle)
			errorutils.WarnOnFail(err, errorutils.WithMsg("error while extracting target title"))

		} else {
			fmt.Println("title was empty")
		}
	} else {
		fmt.Println("Target div not found.")
	}
	return targetTitle.FirstChild.Data, identifier, titler.String() + content.String()
}

func findElement(node *html.Node, typer string, class string) *html.Node {
	defer func() {
		// recover from panic if one occurred. Set err to nil otherwise.
		if recover() != nil {
			fmt.Println("something failed entering debugging mode")
			logrus.SetLevel(logrus.DebugLevel)
			fmt.Println(node)
			fmt.Println(*node)
			var x int
			cleanWarning(node, typer, &x)
			logrus.Debug(fmt.Sprintf("the recovery cleared %d warnings", x))

		}
	}()

	if node.Type == html.ElementNode && node.Data == typer {
		for _, attr := range node.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, class) {
				var y int
				cleanWarning(node, typer, &y)
				logrus.Debug(fmt.Sprintf("the main cleared %d warnings", y))
				return node
			}

		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		found := findElement(child, typer, class)
		if found != nil {
			return found
		}
	}

	return nil
}

func saveFile(content string) {
	header := `<!DOCTYPE html>
<html>
<body>
`
	tailer := `
</body>
</html>`
	filename := selectFile("chapter")
	err := os.WriteFile(filename, []byte(header+content+tailer), 0644)
	errorutils.WarnOnFail(err,
		errorutils.WithLineRef("j2fUdT6"),
		errorutils.WithAltPrint(fmt.Sprintln("Chapter saved to:", filename)))
}

func selectFile(word string) string {
	var filename string
	chapterNum := 1
	for {
		filename = filepath.Join(os.Getenv("HOME"), "Downloads", fmt.Sprintf("%s%04d.html", word, chapterNum))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			break
		}
		chapterNum++
	}
	return filename
}

func saveDoc(doc *html.Node) {
	var content bytes.Buffer
	filename := selectFile("debugnode")
	err := html.Render(&content, doc)
	errorutils.WarnOnFail(err, errorutils.WithLineRef("N25EQuAR0Qg"))

	err2 := os.WriteFile(filename, content.Bytes(), 0644)
	errorutils.WarnOnFail(err2,
		errorutils.WithLineRef("OHDwZtUPEJP"),
		errorutils.WithAltPrint(fmt.Sprintln("Chapter saved to:", filename)))

}

func printMyJS() {
	reminder := `const tbody = document.querySelector('tbody');
const links = [];

for (const tr of tbody.rows) {
    const firstTd = tr.querySelector('td');
    const anchor = firstTd.querySelector('a');
    if (anchor) {
        links.push(anchor.href);
    }
}

const blob = new Blob([links.join('\n')], { type: 'text/plain' });
const url = URL.createObjectURL(blob);
const link = document.createElement('a');
link.href = url;
link.download = 'links.txt';
link.click();
`
	fmt.Println(reminder)
}

func generateRandomString() string {
	length := 5
	charSet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	charSetLength := big.NewInt(int64(len(charSet)))
	for i := 0; i < length; i++ {
		randomIndex, _ := rand.Int(rand.Reader, charSetLength)
		sb.WriteByte(charSet[randomIndex.Int64()])
	}
	return sb.String()
}
