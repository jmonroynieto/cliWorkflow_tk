package main

import (
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"

	"github.com/pydpll/errorutils"
	"golang.org/x/net/html"
)

var filterID string = "ANY2CNyuBxA"
var currentfilter = map[uint32]struct{}{
	2053519342: {},
	862081231:  {},
	101926157:  {},
	242526914:  {},
	1929198708: {},
	1861343925: {},
	3483304230: {},
	1872030291: {},
	1685995898: {},
	90388115:   {},
	1132405187: {},
	1781110435: {},
	1940650954: {},
	1963541962: {},
	1672767817: {},
	3459036400: {},
	3643857914: {},
	643716378:  {},
	3084059268: {},
	3891163734: {},
	679154348:  {},
	2807316473: {},
	2000547447: {},
	211678060:  {},
	824203592:  {},
	1568476032: {},
	3125019371: {},
	3814222657: {},
	4051479283: {},
	486637448:  {},
	1024612907: {},
	3502444588: {},
	540770299:  {},
	3272116394: {},
	1496654099: {},
	3699946697: {},
	3997439741: {},
	3810795681: {},
	2908500774: {},
	2330778139: {},
	2845540295: {},
	4114250433: {},
	1167329149: {},
	3235573041: {},
	313587639:  {},
	2298365139: {},
	484848113:  {},
	1506728389: {},
	3815247533: {},
	1223060675: {},
	3148747835: {},
	2318636733: {},
	1840853303: {},
	714495533:  {},
	2471906766: {},
	1781197856: {},
	3749034810: {},
	1974377751: {},
	1741957812: {},
	3215500148: {},
	3364705970: {},
	2975036390: {},
	2965350011: {},
	1487318402: {},
	3398584058: {},
	257435750:  {},
	2044394885: {},
	3641139632: {},
	2008960439: {},
	3951195258: {},
	3966062797: {},
	2352147149: {},
	1537773700: {},
	1835441243: {},
	1367953591: {},
	1198797474: {},
	3156627556: {},
	1480138083: {},
	2329468885: {},
	1512447916: {},
	710856771:  {},
	3981119862: {},
	2811852940: {},
	656409490:  {},
	495502740:  {},
	686374194:  {},
	3070195408: {},
	3729905087: {},
	2971362601: {},
	3490451415: {},
	2738377501: {},
	3278891293: {},
	3892919337: {},
	911825781:  {},
	499138741:  {},
	2335775082: {},
	2632229980: {},
	853864477:  {},
	4083655406: {},
	2967523524: {},
	389755869:  {},
	1150360322: {},
	542407946:  {},
	1347641307: {},
	1884388298: {},
	1812721871: {},
	2322393393: {},
	3559529403: {},
	3529168240: {},
	3330254909: {},
	2608995630: {},
	1265394201: {},
	4188129229: {},
	2980700007: {},
	3582330237: {},
	2444581192: {},
	971975307:  {},
	213227858:  {},
	3204273152: {},
	330134112:  {},
	2398499966: {},
	3665364060: {},
	1043610494: {},
	2649721380: {},
	1888386524: {},
	1985482058: {},
	1426760275: {},
	3971083095: {},
	3472783108: {},
	2538737113: {},
	216537815:  {},
	1804627140: {},
	89144254:   {},
	1320910879: {},
	4292602804: {},
	3984195585: {},
	2428083245: {},
	3245864696: {},
	768521837:  {},
	1480260908: {},
	1265533230: {},
	3374608022: {},
	4210682988: {},
	1561310304: {},
	98112192:   {},
	3736212041: {},
	348915122:  {},
	2004567466: {},
	1012184156: {},
	4070047812: {},
	108314335:  {},
	4293043627: {},
	3438748018: {},
	2616666474: {},
	2219321436: {},
	1493871062: {},
	3061747301: {},
	164142025:  {},
	356924023:  {},
	74436480:   {},
	998794321:  {},
	622764279:  {},
	3241722151: {},
	1982065308: {},
	4218129493: {},
}

var warnings = []string{"Unauthorized use of content: if you find this story on Amazon, report the violation.", "If you spot this story on Amazon, know that it has been stolen. Report the violation.", "This tale has been pilfered from Royal Road. If found on Amazon, kindly file a report.", "Stolen from its original source, this story is not meant to be on Amazon", "If you come across this story on Amazon, be aware that it has been stolen from Royal Road. Please report it.", "This story has been stolen from Royal Road. If you read it on Amazon, please report it", "Unauthorized use: this story is on Amazon without permission from the author. Report any sightings.", "The narrative has been stolen; if detected on Amazon, report the infringement.", "Stolen from Royal Road, this story should be reported if encountered on Amazon", "If you find this story on Amazon, be aware that it has been stolen. Please report the infringement.", "If you come across this story on Amazon, it's taken without permission from the author. Report it", "If you encounter this story on Amazon, note that it's taken without permission from the author. Report it.", "The story has been illicitly taken; should you find it on Amazon, report the infringement.", "The story has been taken without consent; if you see it on Amazon, report the incident.", "The narrative has been taken without permission. Report any sightings.", "A case of theft: this story is not rightfully on Amazon; if you spot it, report the violation.", "Stolen from its original source, this story is not meant to be on Amazon; report any sightings.", "The author's content has been appropriated; report any instances of this story on Amazon.", "This narrative has been purloined without the author's approval. Report any appearances on Amazon.", "The story has been illicitly taken", "should you find it on Amazon, report the infringement.", "Stolen content alert: this content belongs on Royal Road. Report any occurrences.", "Stolen novel; please report.", "Unauthorized reproduction: this story has been taken without approval."}

func cleanWarning(node *html.Node, blockType string, filteredCount *int) {
	if blockType == "h1" || len(node.Data) > 140 || len(node.Data) < 20 { //average size of a warning is 82ch, long paragraphs are unlikely to be used for this purpose, small strings do not pose that mmuch of a limitation to this filtering option due to how to coverage is calculated but very small ones are more likely to be false positives. Skip large and small.
		goto lookInside
	}
	if node.Type == html.TextNode && isHumanReadableSentence(node.Data) {
		filterCoverage, queryFraction := filterMatchMetrics(node.Data)
		if filterCoverage > 15.0 {
			*filteredCount++
			node.Parent.RemoveChild(node)
			return
		} else if filterCoverage > 10.0 && queryFraction < 40.0 { //grey territory for many hits in the filter which ALSO cover plenty of the query string.
			if shouldHandleMatch(node.Data, filterCoverage, queryFraction) {
				/*
					fugitives list:
					[]string{"It should be possible to revive you, if you want to be alive again."}
				*/
				*filteredCount++
				node.Parent.RemoveChild(node)
				return
			}
		}
	}
lookInside:
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		cleanWarning(c, blockType, filteredCount)
	}
}

// To update code after warnings have been updated, compile, then run updateFilter command in the app and update the filter
func generateFilter() string {
	var set = make(map[uint32]struct{})
	for _, warning := range warnings {
		for _, hash := range hashTextWindows(warning) {
			set[hash] = struct{}{}
		}
	}
	var codeBuilder strings.Builder

	codeBuilder.WriteString("var currentfilter = map[uint32]struct{}{\n")
	for key := range set {
		codeBuilder.WriteString(fmt.Sprintf("%d: {},\n", key))
	}
	codeBuilder.WriteString("}")

	return codeBuilder.String()
}

func filterMatchMetrics(test string) (coverage float32, fraction float32) {

	counter := 0
	hw := hashTextWindows(test)
	for _, testHash := range hw {
		if _, ok := currentfilter[testHash]; ok {
			counter++
		}
	}
	/*
		not a real porcentage since divided by smaller number of warnings and
		not hashes in currentFilter. This metric seems to work better to genrate rules
		during development all negative controls report 0.00% while all positive
		controls have at least 13%
	*/
	return (float32(counter) / float32(len(warnings))) * 100, (float32(counter) / float32(len(hw))) * 100

}

func hashTextWindows(text string) []uint32 {
	words := strings.Split(text, " ")
	windowSize := 2
	hashes := make([]uint32, 0)
	for i := 0; i <= len(words)-windowSize; i++ {
		window := words[i : i+windowSize]
		hashValue := crc32.ChecksumIEEE([]byte(strings.Join(window, " ")))
		hashes = append(hashes, hashValue)
	}
	return hashes
}

func shouldHandleMatch(text string, matchCov float32, matchFract float32) bool {
	fmt.Printf("Found a potential  match (%.2f points, %.2f%% of the query are matches)  for textblock `%s`\n", matchCov, matchFract, text)
	fmt.Print("Do you want to remove? (y/n): ")

	var response string
	fmt.Scanln(&response)

	return strings.ToLower(response) == "y"

}

func isHumanReadableSentence(s string) bool {
	// Regular expression to match strings with at least three non-consecutive spaces
	regexPattern := `^(?:[^\s]*\s){3,}[^\s]*$`

	match, err := regexp.MatchString(regexPattern, s)
	errorutils.WarnOnFailf(err, "Regex failure: %s")
	return match
}

// func containsAny(str string, substrs []string) bool {
// 	for _, substr := range substrs {
// 		if strings.Contains(str, substr) {
// 			return true
// 		}
// 	}
// 	return false
// }

// func containsAnyParallel(str string, substrs []string) bool {
// 	var wg sync.WaitGroup
// 	matchCh := make(chan struct{}, 1) // Buffered channel with capacity 1 for early termination

// 	for _, substr := range substrs {
// 		wg.Add(1)
// 		go func(substr string) {
// 			defer wg.Done()
// 			if strings.Contains(str, substr) {
// 				select {
// 				case matchCh <- struct{}{}:
// 					// Signal early termination by sending a value to the channel
// 				default:
// 					// Channel already closed, early termination has occurred
// 				}
// 			}
// 		}(substr)
// 	}

// 	go func() {
// 		wg.Wait()
// 		close(matchCh) // Close the channel once all goroutines finish
// 	}()

// 	_, ok := <-matchCh // Wait for either a match or channel closure
// 	return ok
// }
