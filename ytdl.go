package ytdl

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"time"

	"github.com/kkdai/youtube/v2"
	"golang.org/x/net/http/httpproxy"
)

var (
	qualities          = []string{"144p", "240p", "360p", "480p", "hd720", "hd1080", "hd1440", "hd2160"}
	downloaderInstance *downloader
	insecureSkipVerify bool // skip TLS server validation
)

// DownloadAudio downloads the audio stream from a youtube video based on its id.
func DownloadAudio(id string) error {
	mimetype := "mp4"
	qualityIndex := 4
	outputQuality := qualities[qualityIndex]
	outputDir := "."
	language := ""

	for qualityIndex >= 0 {
		err := downloadSong(id, outputDir, outputQuality, mimetype, language)
		if err == nil {
			return nil
		}
		if errors.Is(err, errInvalidFormat) {
			if qualityIndex-1 >= 0 {
				qualityIndex--
			} else {
				return err
			}
			outputQuality = qualities[qualityIndex]
			continue
		}
		if err != nil && !errors.Is(err, errInvalidFormat) {
			return err
		}
	}

	if qualityIndex < 0 {
		return errInvalidFormat
	}

	return nil
}

// DownloadVideo downloads a youtube video based on its id.
func DownloadVideo(id string) error {
	if err := checkFFMPEG(); err != nil {
		return err
	}

	mimetype := "mp4"
	qualityIndex := 4
	outputQuality := qualities[qualityIndex]
	outputDir := "."
	language := ""

	for qualityIndex >= 0 {
		err := downloadVideo(id, outputDir, outputQuality, mimetype, language)
		if err == nil {
			return nil
		}
		if errors.Is(err, errInvalidFormat) {
			if qualityIndex-1 >= 0 {
				qualityIndex--
			} else {
				return err
			}
			outputQuality = qualities[qualityIndex]
			continue
		}
		if err != nil && !errors.Is(err, errInvalidFormat) {
			return err
		}
	}

	if qualityIndex < 0 {
		return errInvalidFormat
	}

	return nil
}

func downloadVideo(id, outputDir, outputQuality, mimetype, language string) error {
	video, _, err := getVideoWithFormat(id, outputQuality, mimetype, language)
	if err != nil {
		return err
	}

	log.Println("download to directory", outputDir)

	if err := checkFFMPEG(); err != nil {
		return err
	}
	err = downloaderInstance.DownloadComposite(context.Background(), id+".mp4", video, outputQuality, mimetype, language)
	if err != nil {
		return err
	}

	return nil
}

func downloadSong(id, outputDir, outputQuality, mimetype, language string) error {
	video, _, err := getVideoWithFormat(id, outputQuality, mimetype, language)
	if err != nil {
		return err
	}

	log.Println("download to directory", outputDir)

	if err := checkFFMPEG(); err != nil {
		return err
	}
	err = downloaderInstance.DownloadAudio(context.Background(), id+".mp3", video, outputQuality, mimetype, language)
	if err != nil {
		return err
	}

	return nil
}

func checkFFMPEG() error {
	fmt.Println("check ffmpeg is installed....")
	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		return fmt.Errorf("please check ffmpegCheck is installed correctly")
	}

	return nil
}

func getDownloader() *downloader {
	if downloaderInstance != nil {
		return downloaderInstance
	}

	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	httpTransport := &http.Transport{
		// Proxy: http.ProxyFromEnvironment() does not work. Why?
		Proxy: func(r *http.Request) (uri *url.URL, err error) {
			return proxyFunc(r.URL)
		},
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	youtube.SetLogLevel("info")

	if insecureSkipVerify {
		youtube.Logger.Info("Skip server certificate verification")
		httpTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	downloaderInstance = &downloader{
		OutputDir: ".",
	}
	downloaderInstance.HTTPClient = &http.Client{Transport: httpTransport}

	return downloaderInstance
}

var errInvalidFormat = errors.New("unable to find the specified format")

func getVideoWithFormat(videoID, outputQuality, mimetype, language string) (*youtube.Video, *youtube.Format, error) {
	dl := getDownloader()
	video, err := dl.GetVideo(videoID)
	if err != nil {
		return nil, nil, err
	}

	itag, _ := strconv.Atoi(outputQuality)
	formats := video.Formats

	if language != "" {
		formats = formats.Language(language)
	}
	if mimetype != "" {
		formats = formats.Type(mimetype)
	}
	if outputQuality != "" {
		formats = formats.Quality(outputQuality)
	}
	if itag > 0 {
		formats = formats.Itag(itag)
	}
	if formats == nil {
		return nil, nil, errInvalidFormat
	}

	formats.Sort()

	// select the first format
	return video, &formats[0], nil
}
