package ytdl

import (
	"errors"
	"mime"
	"os"
	"path/filepath"
	"regexp"

	"github.com/kkdai/youtube/v2"
)

const defaultExtension = ".mov"

// Rely on hardcoded canonical mime types, as the ones provided by Go aren't exhaustive [1].
// This seems to be a recurring problem for youtube downloaders, see [2].
// The implementation is based on mozilla's list [3], IANA [4] and Youtube's support [5].
// [1] https://github.com/golang/go/blob/ed7888aea6021e25b0ea58bcad3f26da2b139432/src/mime/type.go#L60
// [2] https://github.com/ZiTAL/youtube-dl/blob/master/mime.types
// [3] https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types
// [4] https://www.iana.org/assignments/media-types/media-types.xhtml#video
// [5] https://support.google.com/youtube/troubleshooter/2888402?hl=en
var canonicals = map[string]string{
	"video/quicktime":  ".mov",
	"video/x-msvideo":  ".avi",
	"video/x-matroska": ".mkv",
	"video/mpeg":       ".mpeg",
	"video/webm":       ".webm",
	"video/3gpp2":      ".3g2",
	"video/x-flv":      ".flv",
	"video/3gpp":       ".3gp",
	"video/mp4":        ".mp4",
	"video/ogg":        ".ogv",
	"video/mp2t":       ".ts",
}

func pickIdealFileExtension(mediaType string) string {
	mediaType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return defaultExtension
	}

	if extension, ok := canonicals[mediaType]; ok {
		return extension
	}

	// Our last resort is to ask the operating system, but these give multiple results and are rarely canonical.
	extensions, err := mime.ExtensionsByType(mediaType)
	if err != nil || extensions == nil {
		return defaultExtension
	}

	return extensions[0]
}

func sanitizeFilename(fileName string) string {
	// Characters not allowed on mac
	//	:/
	// Characters not allowed on linux
	//	/
	// Characters not allowed on windows
	//	<>:"/\|?*

	// Ref https://docs.microsoft.com/en-us/windows/win32/fileio/naming-a-file#naming-conventions

	fileName = regexp.MustCompile(`[:/<>\:"\\|?*]`).ReplaceAllString(fileName, "")
	fileName = regexp.MustCompile(`\s+`).ReplaceAllString(fileName, " ")

	return fileName
}

func getOutputFile(v *youtube.Video, format *youtube.Format, outputFile, outDir string) (string, error) {
	if outputFile == "" {
		outputFile = sanitizeFilename(v.Title)
		outputFile += pickIdealFileExtension(format.MimeType)
	}

	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return "", err
		}
		outputFile = filepath.Join(outDir, outputFile)
	}

	return outputFile, nil
}

func getVideoAudioFormats(v *youtube.Video, quality string, mimetype, language string) (*youtube.Format, *youtube.Format, error) {
	var videoFormats, audioFormats youtube.FormatList

	formats := v.Formats
	if mimetype != "" {
		formats = formats.Type(mimetype)
	}

	videoFormats = formats.Type("video").AudioChannels(0)
	audioFormats = formats.Type("audio")

	if quality != "" {
		videoFormats = videoFormats.Quality(quality)
	}

	if language != "" {
		audioFormats = audioFormats.Language(language)
	}

	if len(videoFormats) == 0 {
		return nil, nil, errors.New("no video format found after filtering")
	}

	if len(audioFormats) == 0 {
		return nil, nil, errors.New("no audio format found after filtering")
	}

	videoFormats.Sort()
	audioFormats.Sort()

	return &videoFormats[0], &audioFormats[0], nil
}
