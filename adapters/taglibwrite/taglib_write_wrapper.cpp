#include <stdlib.h>
#include <string.h>
#include <fstream>
#include <vector>

#define TAGLIB_STATIC
#include <fileref.h>
#include <id3v2tag.h>
#include <attachedpictureframe.h>
#include <mpegfile.h>
#include <tpropertymap.h>
#include <tstring.h>
#include <tstringlist.h>

#include "taglib_write_wrapper.h"

static void put_property(TagLib::PropertyMap &properties, const char *key, const char *value) {
  if (value == nullptr) {
    return;
  }
  TagLib::String v(value, TagLib::String::UTF8);
  if (v.isEmpty()) {
    return;
  }
  TagLib::StringList list;
  list.append(v);
  properties.replace(key, list);
}

static bool attach_cover_mp3(TagLib::MPEG::File *mp3File, const char *coverPath) {
  if (mp3File == nullptr || coverPath == nullptr || *coverPath == '\0') {
    return false;
  }

  std::ifstream image(coverPath, std::ios::binary);
  if (!image) {
    return false;
  }

  std::vector<char> data((std::istreambuf_iterator<char>(image)), std::istreambuf_iterator<char>());
  if (data.empty()) {
    return false;
  }

  auto *tag = mp3File->ID3v2Tag(true);
  if (tag == nullptr) {
    return false;
  }

  auto existing = tag->frameListMap()["APIC"];
  for (auto *frame : existing) {
    tag->removeFrame(frame, true);
  }

  auto *picture = new TagLib::ID3v2::AttachedPictureFrame;
  picture->setType(TagLib::ID3v2::AttachedPictureFrame::FrontCover);
  picture->setMimeType("image/jpeg");
  picture->setDescription("Front cover");
  picture->setPicture(TagLib::ByteVector(data.data(), (unsigned int)data.size()));
  tag->addFrame(picture);
  return true;
}

int taglib_write_comment(const FILENAME_CHAR_T *filename, const char *comment) {
  TagLib::FileRef f(filename, false, TagLib::AudioProperties::Fast);

  if (f.isNull() || f.file() == nullptr) {
    return TAGLIB_ERR_PARSE;
  }

  const char *value = comment != nullptr ? comment : "";
  TagLib::String tagValue(value, TagLib::String::UTF8);

  if (f.tag() != nullptr) {
    f.tag()->setComment(tagValue);
  }

  TagLib::PropertyMap properties = f.file()->properties();
  if (tagValue.isEmpty()) {
    properties.erase("COMMENT");
    properties.erase("comment");
  } else {
    TagLib::StringList list;
    list.append(tagValue);
    properties.replace("COMMENT", list);
    properties.replace("comment", list);
  }
  f.file()->setProperties(properties);

  if (!f.file()->save()) {
    return TAGLIB_ERR_SAVE;
  }

  return 0;
}

int taglib_write_fetched_metadata(const FILENAME_CHAR_T *filename, const char *album, const char *year, const char *genre, const char *recording_mbid, const char *release_mbid, const char *cover_path) {
  TagLib::FileRef f(filename, false, TagLib::AudioProperties::Fast);
  if (f.isNull() || f.file() == nullptr) {
    return TAGLIB_ERR_PARSE;
  }

  if (f.tag() != nullptr) {
    if (album != nullptr && *album != '\0') {
      f.tag()->setAlbum(TagLib::String(album, TagLib::String::UTF8));
    }
    if (genre != nullptr && *genre != '\0') {
      f.tag()->setGenre(TagLib::String(genre, TagLib::String::UTF8));
    }
    if (year != nullptr && *year != '\0') {
      f.tag()->setYear((unsigned int)atoi(year));
    }
  }

  TagLib::PropertyMap properties = f.file()->properties();
  put_property(properties, "ALBUM", album);
  put_property(properties, "DATE", year);
  put_property(properties, "YEAR", year);
  put_property(properties, "GENRE", genre);
  put_property(properties, "MUSICBRAINZ_RECORDINGID", recording_mbid);
  put_property(properties, "MUSICBRAINZ_RELEASEID", release_mbid);
  f.file()->setProperties(properties);

  TagLib::MPEG::File *mp3File = dynamic_cast<TagLib::MPEG::File *>(f.file());
  attach_cover_mp3(mp3File, cover_path);

  if (!f.file()->save()) {
    return TAGLIB_ERR_SAVE;
  }

  return 0;
}

