#define TAGLIB_ERR_PARSE -1
#define TAGLIB_ERR_SAVE -3

#ifdef __cplusplus
extern "C" {
#endif

#ifdef WIN32
#define FILENAME_CHAR_T wchar_t
#else
#define FILENAME_CHAR_T char
#endif

int taglib_write_comment(const FILENAME_CHAR_T *filename, const char *comment);
int taglib_write_fetched_metadata(const FILENAME_CHAR_T *filename, const char *album, const char *year, const char *genre, const char *recording_mbid, const char *release_mbid, const char *cover_path);

#ifdef __cplusplus
}
#endif
