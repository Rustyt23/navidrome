package nativeapi

import "github.com/navidrome/navidrome/adapters/taglib"

type songCommentWriter interface {
	Update(path, comment string) error
}

type taglibSongCommentWriter struct{}

func (taglibSongCommentWriter) Update(path, comment string) error {
	return taglib.WriteComment(path, comment)
}

func newSongCommentWriter() songCommentWriter {
	return taglibSongCommentWriter{}
}
