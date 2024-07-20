package ytdl

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/kkdai/youtube/v2"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

// downloader offers high level functions to download videos into files
type downloader struct {
	youtube.Client
	OutputDir string // optional directory to store the files
}

func (dl *downloader) getOutputFile(v *youtube.Video, format *youtube.Format, outputFile string) (string, error) {
	if outputFile == "" {
		outputFile = sanitizeFilename(v.Title)
		outputFile += pickIdealFileExtension(format.MimeType)
	}

	if dl.OutputDir != "" {
		if err := os.MkdirAll(dl.OutputDir, 0o755); err != nil {
			return "", err
		}
		outputFile = filepath.Join(dl.OutputDir, outputFile)
	}

	return outputFile, nil
}

// Download : Starting download video by arguments.
func (dl *downloader) Download(ctx context.Context, v *youtube.Video, format *youtube.Format, outputFile string) error {
	youtube.Logger.Info(
		"Downloading video",
		"id", v.ID,
		"quality", format.Quality,
		"mimeType", format.MimeType,
	)
	destFile, err := dl.getOutputFile(v, format, outputFile)
	if err != nil {
		return err
	}

	// Create output file
	out, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer out.Close()

	return dl.videoDLWorker(ctx, out, v, format)
}

// DownloadComposite : Downloads audio and video streams separately and merges them via ffmpeg.
func (dl *downloader) DownloadComposite(ctx context.Context, outputFile string, v *youtube.Video, quality string, mimetype, language string) error {
	videoFormat, audioFormat, err1 := getVideoAudioFormats(v, quality, mimetype, language)
	if err1 != nil {
		return err1
	}

	log := youtube.Logger.With("id", v.ID)

	log.Info(
		"Downloading composite video",
		"videoQuality", videoFormat.QualityLabel,
		"videoMimeType", videoFormat.MimeType,
		"audioMimeType", audioFormat.MimeType,
	)

	destFile, err := dl.getOutputFile(v, videoFormat, outputFile)
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(destFile)

	// Create temporary audio file
	audioFile, err := os.Create(path.Join(outputDir, outputFile))
	if err != nil {
		return err
	}
	// defer os.Remove(audioFile.Name())

	log.Debug("Downloading audio file...")
	err = dl.videoDLWorker(ctx, audioFile, v, audioFormat)
	if err != nil {
		return err
	}

	return nil
}

// DownloadAudio : Just downloads audio stream.
func (dl *downloader) DownloadAudio(ctx context.Context, outputFile string, v *youtube.Video, quality string, mimetype, language string) error {
	videoFormat, audioFormat, err1 := getVideoAudioFormats(v, quality, mimetype, language)
	if err1 != nil {
		return err1
	}

	log := youtube.Logger.With("id", v.ID)

	log.Info(
		"Downloading audio",
		"audioQuality", videoFormat.QualityLabel,
		"audioMimeType", audioFormat.MimeType,
	)

	destFile, err := dl.getOutputFile(v, videoFormat, outputFile)
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(destFile)

	audioFile, err := os.Create(path.Join(outputDir, destFile))
	if err != nil {
		return err
	}
	// defer os.Remove(audioFile.Name())

	log.Debug("Downloading audio file...")
	err = dl.videoDLWorker(ctx, audioFile, v, audioFormat)
	if err != nil {
		return err
	}

	return nil
}

type progress struct {
	contentLength     float64
	totalWrittenBytes float64
	downloadLevel     float64
}

func (dl *progress) Write(p []byte) (n int, err error) {
	n = len(p)
	dl.totalWrittenBytes = dl.totalWrittenBytes + float64(n)
	currentPercent := (dl.totalWrittenBytes / dl.contentLength) * 100
	if (dl.downloadLevel <= currentPercent) && (dl.downloadLevel < 100) {
		dl.downloadLevel++
	}
	return
}

func (dl *downloader) videoDLWorker(ctx context.Context, out *os.File, video *youtube.Video, format *youtube.Format) error {
	stream, size, err := dl.GetStreamContext(ctx, video, format)
	if err != nil {
		return err
	}

	prog := &progress{
		contentLength: float64(size),
	}

	// create progress bar
	progress := mpb.New(mpb.WithWidth(64))
	bar := progress.AddBar(
		int64(prog.contentLength),

		mpb.PrependDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
			decor.Percentage(decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name(" ] "),
			decor.EwmaSpeed(decor.UnitKiB, "% .2f", 60),
		),
	)

	reader := bar.ProxyReader(stream)
	mw := io.MultiWriter(out, prog)
	_, err = io.Copy(mw, reader)
	if err != nil {
		return err
	}

	progress.Wait()
	return nil
}
