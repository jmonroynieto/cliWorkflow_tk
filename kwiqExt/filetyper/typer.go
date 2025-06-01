package filetper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"github.com/pydpll/errorutils"
	"github.com/sirupsen/logrus"
)

type FmtType int

const (
	UNKNOWN    FmtType = iota // Failed to identify
	STRUCTURED                // csv, json, xml, etc
	TXT                       // Plain text files (.txt, .log, .md, .ini, .cfg, .csv)
	MEDIA                     // Images, video, audio
	PDF                       // Portable Document Format
	OFFICE                    // Word processor, spreadsheet, presentation files
	ARCHIVE                   // Compressed archives
	SOURCE                    // Scripts, source code, markup, structured data
	BINEXEC                   // Binary executables and libraries
	FONT                      // Font files
	BIOINFO                   //BAM, FASTA, and other biological sequence formats
	OTHER                     // Recognized but not categorized
)

// String representation for the updated enum. special case for directories with invalid enum val (222). Any other value will return "UNMATCHED" which should trigger program failure.
func (ft FmtType) String() string {
	switch ft {
	case TXT:
		return "TXT"
	case STRUCTURED:
		return "STRUCTURED"
	case MEDIA:
		return "MEDIA"
	case PDF:
		return "PDF"
	case OFFICE:
		return "OFFICE"
	case ARCHIVE:
		return "ARCHIVE"
	case SOURCE:
		return "SOURCE"
	case BINEXEC:
		return "BINEXEC"
	case FONT:
		return "FONT"
	case BIOINFO:
		return "BIOINFO"
	case OTHER:
		return "OTHER"
	case FmtType(222):
		return "DIR*" //special printing for directories because they are not formally recognized in SPURI where these enum is used in files only.
	case UNKNOWN:
		return "UNKNOWN"
	default:
		return "UNMATCHED" //this should never happen, test for this case and generate catastrophic error asking to fix.
	}
}
func DetermineFMTtype(filePath string) (FmtType, error) {
	mappedType := mapExtensionToFmtType(filePath)
	if mappedType == UNKNOWN {
		return mimeTypeContent(filePath)
	}
	return mappedType, nil
}

// primary method
func mapExtensionToFmtType(path string) FmtType {
	sanitizedPath := path
	if strings.HasPrefix(path, ".") {
		sanitizedPath = path[1:]
	}
	ext := strings.ToLower(filepath.Ext(sanitizedPath))
	if ext == "" || ext == "." {
		return UNKNOWN
	}
	switch ext {
	case ".txt", ".log", ".md", ".markdown", ".ini", ".cfg", ".conf", ".text", "json": // Config formats
		return TXT
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".svg", ".heic", ".heif", // Images
		".mp4", ".avi", ".mov", ".wmv", ".mkv", ".flv", ".webm", ".mpg", ".mpeg", ".m4b", "vob", // Video
		".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".opus", "aif", "aiff": // Audio
		return MEDIA
	case ".pdf":
		return PDF
	case ".doc", ".docx", ".rtf", // Word processing
		".xls", ".xlsx", ".xlsm", // Spreadsheets
		".ppt", ".pptx", "ppsx", // Presentations
		".odt", ".ods", ".odp", // OpenDocument
		".asd",         // autosave
		".msg", ".eml", // Email
		".ai", ".eps", ".afdesign", ".affont", ".afphoto", ".afpub", // Vector graphics except SVG
		"psd":
		return OFFICE
	case ".zip", ".tar", ".gz", ".tgz", ".rar", ".7z", ".bz2", ".xz", ".war", ".ear":
		next := path[:len(path)-len(ext)]
		ext2 := strings.ToLower(filepath.Ext(next))
		if ext2 == "" || ext2 == "." {
			return ARCHIVE
		}
		if bioinfoCK(ext2) {
			return BIOINFO
		}
		return ARCHIVE
	// Source code
	case ".go", ".java", ".class", ".jar", ".c", ".cpp", ".h", ".hpp", ".cs", ".swift", ".m", ".mm", ".zig", ".odin", "rs", // Compiled languages
		".hh", ".cc",
		".py", ".pyc", ".pyd", ".pyo", // Python (source, compiled)
		".js", ".mjs", ".cjs", // JavaScript
		".ts", ".tsx", // TypeScript
		".sh", ".bash", ".zsh", ".csh", ".ksh", ".fish", // Shell scripts
		".bat", ".cmd", // Windows batch
		".ps1", ".psm1", // PowerShell
		".rb", ".php", ".pl", ".lua", // Other script languages
		".html", ".htm", ".xhtml", // Markup
		".css", ".scss", ".sass", // Stylesheets
		".sql", ".ddl", ".dml", // Database scripts
		".nf", ".smk": // bioinfo workflow
		return SOURCE
	case ".mod", ".sum":
		switch path {
		case "go.mod", "go.sum":
			return SOURCE
		}
		return UNKNOWN
	case ".exe", ".dll", ".so", ".dylib", // Common libs/executables
		".app",         // macOS Application bundle (directory, but ext mapping might be useful)
		".msi",         // Windows installer
		".deb", ".rpm", // Linux packages
		".bin", // Generic binary, often executable or firmware
		".elf", ".lib":
		return BINEXEC

	case ".ttf", ".otf", ".woff", ".woff2", ".eot":
		return FONT

	case ".iso", ".img", ".vdi", ".vhd", ".vmdk", ".dmg", ".bbolt", ".cayley", "db":
		return ARCHIVE
	default:
		if bioinfoCK(ext) {
			return BIOINFO
		}
		logrus.Debugf("Extension '%s' not specifically mapped, checking through content.", ext)
		return UNKNOWN
	}
}

func bioinfoCK(ext string) bool {
	switch ext {
	case ".fasta", ".fastq", ".fa", ".fq", ".fas", ".ffn", ".faa",
		".fna", ".fsa", ".aln", ".fai", ".bai", ".crai", ".maf",
		".clustal", ".phy", ".phylip", ".nwk", ".newick",
		".sam", ".bam", ".cram", ".vcf", ".gff", ".gff3", ".gtf", ".gff2", ".bed",
		".pbd", ".k2d", "dmp", ".hgsketch", ".mash", ".mashsketch":
		return true
	default:
		return false
	}
}

// fallback method
func mimeTypeContent(filePath string) (FmtType, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return UNKNOWN, errorutils.NewReport("File not found "+filePath, "", errorutils.WithInner(err))
		}
		if os.IsPermission(err) {
			return UNKNOWN, errorutils.NewReport("Permission denied "+filePath, "", errorutils.WithInner(err))
		}
		return UNKNOWN, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// byte header
	kind := HeaderTest(file)

	if kind == types.Unknown {
		logrus.Debugf("Header match inconclusive for %s. Falling back to extension.", filePath)
		return UNKNOWN, nil
	}
	return mapKindToFmtType(kind), nil
}

func HeaderTest(file *os.File) types.Type {
	//if file is directory return fake directory type
	if x, _ := file.Stat(); x.IsDir() {
		return types.Type{
			MIME: types.MIME{
				Value:   "noHeader/DIR*",
				Subtype: "DIR*",
				Type:    "noHeader",
			},
		}
	}

	header := make([]byte, 300)
	n, readErr := file.Read(header)
	if readErr != nil && readErr != io.EOF {
		logrus.Warnf("Failed to read header from %s: %v", file.Name(), readErr)
		return types.Unknown
	}

	if n == 0 {
		logrus.Debugf("File %s is empty, classifying as TXT .", file.Name())
		ext := filepath.Ext(file.Name())

		return types.Type{
			MIME: types.MIME{
				Value:   "text/plain",
				Subtype: "plain",
				Type:    "text",
			},
			Extension: ext,
		}
	}

	kind, matchErr := filetype.Match(header[:n])
	if matchErr != nil {
		logrus.Errorf("Failed to match header from %s: %v", file.Name(), matchErr)
	}
	if kind == types.Unknown && isMostlyASCII(header[:n]) {
		kind = types.Type{
			MIME: types.MIME{
				Value:   "text/plain",
				Subtype: "plain",
				Type:    "text",
			},
			Extension: filepath.Ext(file.Name()[1:]), // strip leading dot from hidden files
		}
	}

	return kind
}

func isMostlyASCII(data []byte) bool {
	var asciiCount int
	for _, b := range data {
		if b >= 32 && b <= 126 {
			asciiCount++
		}
	}
	return float64(asciiCount)/float64(len(data)) >= 0.81
}

func mapKindToFmtType(kind types.Type) FmtType {
	mime := kind.MIME.Value
	ext := kind.Extension

	switch {
	case mime == "application/pdf":
		return PDF
	case strings.HasPrefix(mime, "image/"),
		strings.HasPrefix(mime, "video/"),
		strings.HasPrefix(mime, "audio/"):
		return MEDIA
	case mime == "text/xml",
		mime == "application/xml",
		mime == "text/csv",
		mime == "text/tab-separated-values":
		return STRUCTURED
	case mime == "application/msword", // .doc
		mime == "application/vnd.ms-excel",
		mime == "application/vnd.ms-powerpoint",
		mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		mime == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		mime == "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		mime == "application/vnd.oasis.opendocument.text",
		mime == "application/vnd.oasis.opendocument.spreadsheet",
		mime == "application/vnd.oasis.opendocument.presentation",
		mime == "application/vnd.ms-outlook":
		return OFFICE
	case strings.HasPrefix(mime, "application/x-tar"),
		mime == "application/zip",
		mime == "application/vnd.rar",
		mime == "application/x-rar-compressed",
		mime == "application/gzip",
		mime == "application/x-bzip2",
		mime == "application/x-7z-compressed",
		mime == "application/x-xz":
		return ARCHIVE

	// SOURCE (Scripts, Code, Markup, Structured Data)
	case mime == "text/html",
		mime == "text/css",
		mime == "application/javascript", // .js
		mime == "application/json",
		mime == "application/xml",
		mime == "application/sql",
		mime == "application/x-sh",     // Shell script (often identified as text/plain too)
		mime == "application/x-python", // Python (often text/plain)
		mime == "application/x-perl",   // Perl (often text/plain)
		mime == "application/x-php",    // PHP (often text/plain)
		mime == "text/x-shellscript",
		mime == "text/x-python",
		mime == "text/x-java-source", // .java (often text/plain)
		mime == "text/x-c",           // C/C++ source (often text/plain)
		mime == "text/x-script.perl",
		mime == "text/x-script.phyton": // Common typo, but filetype might use it
		return SOURCE
	case mime == "text/plain":
		return TXT
	case mime == "application/x-executable",
		mime == "application/vnd.microsoft.portable-executable", // PE files (.exe, .dll)
		mime == "application/x-elf",                             // ELF (Linux)
		mime == "application/x-mach-binary":                     // Mach-O (macOS, iOS)
		return BINEXEC
	case strings.HasPrefix(mime, "application/font-"), // Catches woff, woff2, etc.
		strings.HasPrefix(mime, "application/x-font-"),
		mime == "application/vnd.ms-opentype", // OTF/TTF
		mime == "font/ttf",
		mime == "font/otf":
		return FONT

	// If recognized by header but doesn't fit above categories, classify as OTHER
	default:
		logrus.Debugf("Header MIME '%s' (Ext: %s) recognized but not specifically categorized, mapping to OTHER.", mime, ext)
		return OTHER
	}
}
