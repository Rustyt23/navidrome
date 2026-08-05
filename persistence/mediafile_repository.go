package persistence

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/adapters/taglibwrite"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
	"github.com/pocketbase/dbx"
)

type mediaFileRepository struct {
	sqlRepository
}

var writeMediaFileComment = taglibwrite.WriteComment

type dbMediaFile struct {
	*model.MediaFile `structs:",flatten"`
	Participants     string `structs:"-" json:"-"`
	Tags             string `structs:"-" json:"-"`
	// These are necessary to map the correct names (rg_*) to the correct fields (RG*)
	// without using `db` struct tags in the model.MediaFile struct
	RgAlbumGain *float64 `structs:"-" json:"-"`
	RgAlbumPeak *float64 `structs:"-" json:"-"`
	RgTrackGain *float64 `structs:"-" json:"-"`
	RgTrackPeak *float64 `structs:"-" json:"-"`
	// Joined from media_file_loudness. Aliased with a `loudness_` prefix so the
	// join can never collide with a media_file column.
	LoudnessStatus           string     `structs:"-" json:"-"`
	LoudnessVerdict          string     `structs:"-" json:"-"`
	LoudnessAction           string     `structs:"-" json:"-"`
	LoudnessPhase            int        `structs:"-" json:"-"`
	LoudnessDecision         string     `structs:"-" json:"-"`
	LoudnessWasException     bool       `structs:"-" json:"-"`
	LoudnessLufsBefore       *float64   `structs:"-" json:"-"`
	LoudnessLufsAfter        *float64   `structs:"-" json:"-"`
	LoudnessGainApplied      *float64   `structs:"-" json:"-"`
	LoudnessTpBefore         *float64   `structs:"-" json:"-"`
	LoudnessTpAfter          *float64   `structs:"-" json:"-"`
	LoudnessLraBefore        *float64   `structs:"-" json:"-"`
	LoudnessLraAfter         *float64   `structs:"-" json:"-"`
	LoudnessNullResidual     *float64   `structs:"-" json:"-"`
	LoudnessCodecBefore      string     `structs:"-" json:"-"`
	LoudnessBitrateBefore    int        `structs:"-" json:"-"`
	LoudnessSampleRateBefore int        `structs:"-" json:"-"`
	LoudnessBitDepthBefore   int        `structs:"-" json:"-"`
	LoudnessChannelsBefore   int        `structs:"-" json:"-"`
	LoudnessDurationBefore   float64    `structs:"-" json:"-"`
	LoudnessSizeBefore       int64      `structs:"-" json:"-"`
	LoudnessArtBefore        bool       `structs:"-" json:"-"`
	LoudnessArtAfter         bool       `structs:"-" json:"-"`
	LoudnessCodecAfter       string     `structs:"-" json:"-"`
	LoudnessBitrateAfter     int        `structs:"-" json:"-"`
	LoudnessSampleRateAfter  int        `structs:"-" json:"-"`
	LoudnessBitDepthAfter    int        `structs:"-" json:"-"`
	LoudnessChannelsAfter    int        `structs:"-" json:"-"`
	LoudnessDurationAfter    float64    `structs:"-" json:"-"`
	LoudnessSizeAfter        int64      `structs:"-" json:"-"`
	LoudnessHasBackup        bool       `structs:"-" json:"-"`
	LoudnessError            string     `structs:"-" json:"-"`
	LoudnessAnalyzedAt       *time.Time `structs:"-" json:"-"`
	LoudnessRestoredAt       *time.Time `structs:"-" json:"-"`
}

// loudnessAuditColumns are the media_file_loudness columns joined into media
// file queries, aliased with a `loudness_` prefix.
//
// The join is a LEFT JOIN, so every column is NULL for tracks that have never
// been analyzed. Columns scanned into non-pointer fields must therefore be
// coalesced to a zero value; the rest stay nullable on purpose, because "not
// measured" and "measured as zero" are different things.
var loudnessAuditColumns = map[string]string{
	"status":             "''",
	"verdict":            "''",
	"action":             "''",
	"codec_before":       "''",
	"error":              "''",
	"decision":           "''",
	"was_exception":      "0",
	"phase":              "-1",
	"bitrate_before":     "0",
	"sample_rate_before": "0",
	"bit_depth_before":   "0",
	"channels_before":    "0",
	"duration_before":    "0",
	"size_before":        "0",
	"art_before":         "0",
	"art_after":          "0",
	"codec_after":        "''",
	"bitrate_after":      "0",
	"sample_rate_after":  "0",
	"bit_depth_after":    "0",
	"channels_after":     "0",
	"duration_after":     "0",
	"size_after":         "0",
	"has_backup":         "0",
	// Nullable: no default
	"lufs_before":   "",
	"lufs_after":    "",
	"gain_applied":  "",
	"tp_before":     "",
	"tp_after":      "",
	"lra_before":    "",
	"lra_after":     "",
	"null_residual": "",
	"analyzed_at":   "",
	"restored_at":   "",
}

func loudnessAuditSelectColumns() []string {
	cols := make([]string, 0, len(loudnessAuditColumns))
	for name, zero := range loudnessAuditColumns {
		if zero == "" {
			cols = append(cols, fmt.Sprintf("media_file_loudness.%s as loudness_%s", name, name))
			continue
		}
		cols = append(cols, fmt.Sprintf("coalesce(media_file_loudness.%s, %s) as loudness_%s", name, zero, name))
	}
	slices.Sort(cols) // stable column order for query caching
	return cols
}

// toAudit rebuilds the audit record from the joined columns. Returns nil when
// the track has never been analyzed (no row in media_file_loudness).
func (m *dbMediaFile) toAudit() *model.LoudnessAudit {
	if m.LoudnessAnalyzedAt == nil && m.LoudnessStatus == "" {
		return nil
	}
	audit := &model.LoudnessAudit{
		MediaFileID:      m.ID,
		Status:           m.LoudnessStatus,
		Verdict:          m.LoudnessVerdict,
		Action:           m.LoudnessAction,
		Phase:            m.LoudnessPhase,
		Decision:         m.LoudnessDecision,
		WasException:     m.LoudnessWasException,
		LufsBefore:       m.LoudnessLufsBefore,
		LufsAfter:        m.LoudnessLufsAfter,
		GainApplied:      m.LoudnessGainApplied,
		TpBefore:         m.LoudnessTpBefore,
		TpAfter:          m.LoudnessTpAfter,
		LraBefore:        m.LoudnessLraBefore,
		LraAfter:         m.LoudnessLraAfter,
		NullResidual:     m.LoudnessNullResidual,
		CodecBefore:      m.LoudnessCodecBefore,
		BitrateBefore:    m.LoudnessBitrateBefore,
		SampleRateBefore: m.LoudnessSampleRateBefore,
		BitDepthBefore:   m.LoudnessBitDepthBefore,
		ChannelsBefore:   m.LoudnessChannelsBefore,
		DurationBefore:   m.LoudnessDurationBefore,
		SizeBefore:       m.LoudnessSizeBefore,
		ArtBefore:        m.LoudnessArtBefore,
		ArtAfter:         m.LoudnessArtAfter,
		CodecAfter:       m.LoudnessCodecAfter,
		BitrateAfter:     m.LoudnessBitrateAfter,
		SampleRateAfter:  m.LoudnessSampleRateAfter,
		BitDepthAfter:    m.LoudnessBitDepthAfter,
		ChannelsAfter:    m.LoudnessChannelsAfter,
		DurationAfter:    m.LoudnessDurationAfter,
		SizeAfter:        m.LoudnessSizeAfter,
		HasBackup:        m.LoudnessHasBackup,
		Error:            m.LoudnessError,
		RestoredAt:       m.LoudnessRestoredAt,
	}
	if m.LoudnessAnalyzedAt != nil {
		audit.AnalyzedAt = *m.LoudnessAnalyzedAt
	}
	return audit
}

func (m *dbMediaFile) PostScan() error {
	m.RGTrackGain = m.RgTrackGain
	m.RGTrackPeak = m.RgTrackPeak
	m.RGAlbumGain = m.RgAlbumGain
	m.RGAlbumPeak = m.RgAlbumPeak
	m.MediaFile.LoudnessAudit = m.toAudit()
	var err error
	m.MediaFile.Participants, err = unmarshalParticipants(m.Participants)
	if err != nil {
		return fmt.Errorf("parsing media_file from db: %w", err)
	}
	if m.Tags != "" {
		m.MediaFile.Tags, err = unmarshalTags(m.Tags)
		if err != nil {
			return fmt.Errorf("parsing media_file from db: %w", err)
		}
		m.Genre, m.Genres = m.MediaFile.Tags.ToGenres()
	}
	return nil
}

func (m *dbMediaFile) PostMapArgs(args map[string]any) error {
	fullText := []string{m.FullTitle(), m.Album, m.Artist, m.AlbumArtist,
		m.SortTitle, m.SortAlbumName, m.SortArtistName, m.SortAlbumArtistName, m.DiscSubtitle}
	participantNames := m.MediaFile.Participants.AllNames()
	fullText = append(fullText, participantNames...)
	args["full_text"] = formatFullText(fullText...)
	args["search_participants"] = strings.Join(participantNames, " ")
	args["search_normalized"] = normalizeForFTS(m.FullTitle(), m.Album, m.Artist, m.AlbumArtist)
	args["tags"] = marshalTags(m.MediaFile.Tags)
	args["participants"] = marshalParticipants(m.MediaFile.Participants)
	return nil
}

type dbMediaFiles []dbMediaFile

func (m dbMediaFiles) toModels() model.MediaFiles {
	return slice.Map(m, func(mf dbMediaFile) model.MediaFile { return *mf.MediaFile })
}

func NewMediaFileRepository(ctx context.Context, db dbx.Builder) model.MediaFileRepository {
	r := &mediaFileRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "media_file"
	r.registerModel(&model.MediaFile{}, mediaFileFilter())
	r.setSortMappings(map[string]string{
		"title":          "order_title",
		"artist":         "order_artist_name, order_album_name, release_date, disc_number, track_number",
		"album_artist":   "order_album_artist_name, order_album_name, release_date, disc_number, track_number",
		"album":          "order_album_name, album_id, disc_number, track_number, order_artist_name, title",
		"fetched":        "(media_file.has_cover_art or media_file.cover_path <> '')",
		"random":         "random",
		"created_at":     "media_file.created_at",
		"recently_added": mediaFileRecentlyAddedSort(),
		"starred_at":     "starred, starred_at",
		"genre":          "genre",
		"comment":        "comment",
		"lufs":           mediaFileLufsSort(),
		"rated_at":       "rating, rated_at",
		// Loudness audit (joined from media_file_loudness)
		"loudness_verdict":   "media_file_loudness.verdict",
		"loudness_phase":     "media_file_loudness.phase",
		"loudness_decision":  "media_file_loudness.decision",
		"loudness_status":    "media_file_loudness.status",
		"loudness_action":    "media_file_loudness.action",
		"lufs_before":        "media_file_loudness.lufs_before",
		"lufs_after":         "media_file_loudness.lufs_after",
		"gain_applied":       "media_file_loudness.gain_applied",
		"tp_before":          "media_file_loudness.tp_before",
		"tp_after":           "media_file_loudness.tp_after",
		"lra_before":         "media_file_loudness.lra_before",
		"lra_after":          "media_file_loudness.lra_after",
		"null_residual":      "media_file_loudness.null_residual",
		"bitrate_before":     "media_file_loudness.bitrate_before",
		"sample_rate_before": "media_file_loudness.sample_rate_before",
		"analyzed_at":        "media_file_loudness.analyzed_at",
	})
	return r
}

// mediaFileLufsTagSort reads the loudness a song carries in its own tags.
//
// Only usable on its own where media_file_loudness is not joined. A song this
// server normalized has no such tag: the value is written to the tags column
// but never into the file, so the next scan - which rebuilds that column from
// the file - drops it. Whatever this finds therefore came from outside.
// loudnessOutcomeFilter selects songs by how close they ended up to the target.
//
// The loudness read is whatever the song measures now - lufs_after once it has
// been rewritten, lufs_before while it has only been measured. A song that was
// already on target and never opened counts as on target: it did not need
// processing to get there.
// loudnessExceptionFilter lists the tracks that genuinely needed a person.
//
// A refusal is only worth listing when correcting the track was worth
// attempting. Measured on a real library, seven of ten refusals sat within half
// a decibel of target: each had been gained by a fraction, re-encoded, measured,
// found to have sprung its peak over the ceiling, and thrown away - leaving the
// file exactly as it started and a row asking someone to decide about a
// difference nobody can hear. Those are not exceptions, they are noise, and a
// review page that is mostly noise stops being read.
// effectiveLoudnessTolerance mirrors the engine's own fallback: an unset or
// nonsensical tolerance means the default, not zero. Reading the raw setting
// here while the engine reads the effective one would have the page counting
// against a band the optimiser never used.
func effectiveLoudnessTolerance(tolerance float64) float64 {
	if tolerance <= 0 {
		return conf.DefaultLoudnessNormalizationTolerance
	}
	return tolerance
}

// LoudnessExceptionFilter is what the list filters by, exported so the library
// summary counts the same rows. The summary used to carry its own copy of this
// expression, which drifted the moment the rule changed: the page stopped
// listing the tracks left alone at the wider tolerance and the headline went on
// counting them as exceptions.
func LoudnessExceptionFilter() Sqlizer { return loudnessExceptionFilter("", nil) }

// LoudnessLevelTwoFilter selects tracks held to the wider tolerance: near enough
// to target that correcting them was judged not worth a re-encode, so they were
// left untouched rather than sent to the client.
func LoudnessLevelTwoFilter() Sqlizer {
	options := conf.Server.Scanner.LoudnessNormalization
	tolerance := effectiveLoudnessTolerance(options.Tolerance)
	measured := "coalesce(media_file_loudness.lufs_after, media_file_loudness.lufs_before)"
	withinBand := Expr(
		fmt.Sprintf("%s is not null and abs(%s - ?) <= ?", measured, measured),
		options.TargetLUFS, loudnessLeaveAloneToleranceDB)
	outsideOrdinary := Expr(
		fmt.Sprintf("abs(%s - ?) > ?", measured), options.TargetLUFS, tolerance)

	// Defined as the complement of the exceptions list rather than by
	// enumerating the ways a track gets here. There turned out to be a third:
	// a track gained as far as its peaks allowed, accepted, and still a fraction
	// short. Listing the routes meant that one fell through every category and
	// the panel silently failed to account for it.
	//
	// Built from the exception filter itself so the two cannot drift apart: a
	// track in this band is level two exactly when nobody has to act on it.
	sql, args, err := loudnessExceptionFilter("", nil).ToSql()
	if err != nil {
		// Nothing usable to negate, so claim nothing rather than claim
		// everything.
		return Expr("1 = 0")
	}
	return And{withinBand, outsideOrdinary, Expr("not ("+sql+")", args...)}
}

func loudnessExceptionFilter(_ string, _ any) Sqlizer {
	options := conf.Server.Scanner.LoudnessNormalization
	measured := "coalesce(media_file_loudness.lufs_after, media_file_loudness.lufs_before)"
	// Far enough from target that the attempt was worth making, so its failure
	// is worth reporting.
	worthListing := Expr(
		fmt.Sprintf("%s is null or abs(%s - ?) > ?", measured, measured),
		options.TargetLUFS, loudnessLeaveAloneToleranceDB)

	return And{
		// Never list a track the engine deliberately left alone.
		NotEq{"media_file_loudness.phase": loudnessPhaseCloseEnough},
		Or{
			Eq{"media_file_loudness.was_exception": true},
			Eq{"media_file_loudness.phase": model.LoudnessPhaseReview},
			And{Eq{"media_file_loudness.action": model.LoudnessActionRefused}, worthListing},
			NotEq{"media_file_loudness.decision": ""},
		},
	}
}

// Mirrors core/loudness, which this layer cannot import.
const (
	loudnessPhaseCloseEnough      = 4
	loudnessLeaveAloneToleranceDB = 0.5
)

// Exported so the library summary counts with exactly the expression the list
// filters by. Two copies of this arithmetic would be two chances for the
// headline to disagree with the table someone clicks into to check it.
func LoudnessOutcomeFilter(outcome string) Sqlizer { return loudnessOutcomeFilter("", outcome) }

func loudnessOutcomeFilter(_ string, value any) Sqlizer {
	options := conf.Server.Scanner.LoudnessNormalization
	tolerance := effectiveLoudnessTolerance(options.Tolerance)
	measured := "coalesce(media_file_loudness.lufs_after, media_file_loudness.lufs_before)"

	build := func(one string) Sqlizer {
		switch one {
		case "on_target":
			return Expr(fmt.Sprintf("%s is not null and abs(%s - ?) <= ?", measured, measured),
				options.TargetLUFS, tolerance)
		case "short":
			return Expr(fmt.Sprintf("%s is not null and abs(%s - ?) > ?", measured, measured),
				options.TargetLUFS, tolerance)
		case "not_measured":
			return Expr(fmt.Sprintf("%s is null", measured))
		}
		return nil
	}

	// The UI sends an array, because the filter allows more than one at a time.
	var wanted []string
	switch v := value.(type) {
	case string:
		wanted = []string{v}
	case []string:
		wanted = v
	case []any:
		for _, item := range v {
			wanted = append(wanted, fmt.Sprint(item))
		}
	default:
		return nil
	}

	any := Or{}
	for _, one := range wanted {
		if clause := build(one); clause != nil {
			any = append(any, clause)
		}
	}
	if len(any) == 0 {
		return nil
	}
	return any
}

// The rejections rejection() writes in core/loudness, matched by their opening
// words.
//
// This layer does not import core/loudness, so the tie between these prefixes
// and the strings that produce them is by convention. It fails safe: an error
// shape not listed here matches no category at all rather than being filed
// under the wrong one, so a song can go missing from a filtered view but never
// appears under a cause that is not its own. TestLoudnessReasonFilter pins the
// real strings so a reworded error breaks a test rather than a page.
const (
	rejectedByPeak     = "true peak %"
	rejectedByFormat   = "output was not format-identical%"
	rejectedByLoudness = "landed at %"
)

func LoudnessReasonFilter(reason string) Sqlizer { return loudnessReasonFilter("", reason) }

// Why a song is on the exceptions page.
//
// Grouping by cause is what turns that page from a list into a queue: five
// songs refused for the same reason are one decision, not five, and without
// this the only way to act on them together is to pick them out by eye and
// hope none was missed.
//
// The categories mirror ReasonField in ui/src/lufs2/Lufs2Fields.jsx, precedence
// included - a refusal is reported as a refusal even when the row also carries
// a decision or a processed status. If the two ever disagree the filter selects
// rows whose visible reason is something else, which is worse than selecting
// nothing, so they are pinned together by test.
func loudnessReasonFilter(_ string, value any) Sqlizer {
	// coalesced because the join is a LEFT one: a song with no audit row at all
	// has NULL here, and NULL != 'refused' is NULL rather than true, which
	// would quietly drop those rows from every negated category.
	action := "coalesce(media_file_loudness.action, '')"
	status := "coalesce(media_file_loudness.status, '')"
	refused := Expr(action+" = ?", model.LoudnessActionRefused)
	notRefused := Expr(action+" <> ?", model.LoudnessActionRefused)
	errorLike := func(prefix string) Sqlizer {
		return Like{"coalesce(media_file_loudness.error, '')": prefix}
	}

	build := func(one string) Sqlizer {
		switch one {
		case "peak_over":
			return And{refused, errorLike(rejectedByPeak)}
		case "format_changed":
			return And{refused, errorLike(rejectedByFormat)}
		case "missed_target":
			return And{refused, errorLike(rejectedByLoudness)}
		case "peaks_trimmed":
			return And{
				notRefused,
				Expr(status+" = ?", model.LoudnessStatusProcessed),
				Expr(action+" = ?", model.LoudnessActionLimited),
			}
		case "needs_trade":
			// What is left once the two handled cases are removed: the song the
			// engine will not decide for you. Defined by exclusion for the same
			// reason the level-two filter is - listing the ways in missed one
			// and left rows belonging to no category at all.
			return And{
				notRefused,
				Or{
					Expr(status+" <> ?", model.LoudnessStatusProcessed),
					Expr(action+" <> ?", model.LoudnessActionLimited),
				},
			}
		}
		return nil
	}

	var wanted []string
	switch v := value.(type) {
	case string:
		wanted = []string{v}
	case []string:
		wanted = v
	case []any:
		for _, item := range v {
			wanted = append(wanted, fmt.Sprint(item))
		}
	default:
		return nil
	}

	any := Or{}
	for _, one := range wanted {
		if clause := build(one); clause != nil {
			any = append(any, clause)
		}
	}
	if len(any) == 0 {
		return nil
	}
	return any
}

func mediaFileLufsTagSort() string {
	return "cast(coalesce(" +
		"json_extract(tags, '$.loudnorm_final_lufs[0].value'), " +
		"json_extract(tags, '$.final_lufs[0].value'), " +
		"json_extract(tags, '$.finallufs[0].value'), " +
		"json_extract(tags, '$.lufs[0].value')" +
		") as real)"
}

// mediaFileLufsSort orders by the loudness a song actually has now.
//
// The audit comes first and the tags are the fallback, matching what the UI
// displays - otherwise the column and the sort disagree and the list looks
// randomly ordered. lufs_after is the measurement of the file as it stands
// after processing; lufs_before is that same measurement for a song only ever
// analysed. Requires the media_file_loudness join, which selectMediaFile makes.
func mediaFileLufsSort() string {
	return "(coalesce(media_file_loudness.lufs_after, media_file_loudness.lufs_before, " +
		mediaFileLufsTagSort() + "))"
}

var mediaFileFilter = sync.OnceValue(func() map[string]filterFunc {
	filters := map[string]filterFunc{
		"id":          idFilter("media_file"),
		"title":       fullTextFilter("media_file", "mbz_recording_id", "mbz_release_track_id"),
		"starred":     annotationBoolFilter("starred"),
		"has_rating":  annotationBoolFilter("rating"),
		"genre_id":    tagIDFilter,
		"missing":     booleanFilter,
		"hascoverart": func(_ string, value any) Sqlizer { return booleanFilter("media_file.has_cover_art", value) },
		"fetched": func(_ string, value any) Sqlizer {
			if isTrue(value) {
				return Or{Eq{"media_file.has_cover_art": true}, NotEq{"media_file.cover_path": ""}}
			}
			return And{Eq{"media_file.has_cover_art": false}, Eq{"media_file.cover_path": ""}}
		},
		// "Left as-is" is not a stored verdict. It is what the page calls a
		// track a run built a file for and then refused, which is recorded on
		// the action instead. Offering it here keeps the filter matching the
		// column, which is the only place anyone reads these names - a filter
		// missing an outcome the column displays is worse than useless, because
		// the rows are visibly there and cannot be narrowed to.
		"loudness_verdict": func(_ string, value any) Sqlizer {
			var stored []string
			refused := false
			for _, v := range filterStrings(value) {
				if v == leftAsIsVerdict {
					refused = true
					continue
				}
				stored = append(stored, v)
			}
			byAction := Eq{"media_file_loudness.action": model.LoudnessActionRefused}
			switch {
			case refused && len(stored) > 0:
				return Or{Eq{"media_file_loudness.verdict": stored}, byAction}
			case refused:
				return byAction
			default:
				return eqFilter("media_file_loudness.verdict", value)
			}
		},
		"loudness_status": func(_ string, value any) Sqlizer {
			return eqFilter("media_file_loudness.status", value)
		},
		"loudness_phase": func(_ string, value any) Sqlizer {
			return eqFilter("media_file_loudness.phase", value)
		},
		"loudness_decision": func(_ string, value any) Sqlizer {
			return eqFilter("media_file_loudness.decision", value)
		},
		// Every song whose handling was not routine, and it stays here once it
		// qualifies rather than dropping off the moment it is dealt with. The
		// list is what someone reviews, answers to a client from, and restores
		// out of - none of which works if a song disappears the instant it is
		// processed.
		//
		// Four ways to qualify: the target needs the peaks cut by enough to be
		// heard, which is the client's call and not ours; a run built a file,
		// judged it unfit and kept the original; the peaks were trimmed, so the
		// audio was altered however slightly; or a person made a decision about
		// it. Phase 2 is loudness.PhaseReview - the phases live in
		// core/loudness, which this layer does not import.
		// Tracks that needed human attention, and stay listed afterwards so the
		// admin keeps a permanent record of them.
		//
		// The latch is the real answer; the live conditions alongside it catch
		// rows written before it existed, and any that somehow miss it. Note
		// that action = 'limited' is deliberately absent: a small automatic peak
		// trim records 'limited' too, and matching on it listed several dozen
		// ordinary successes as exceptions.
		"loudness_exception": loudnessExceptionFilter,
		// Where a song ended up relative to the target, which is a different
		// question from whether the file was harmed and the one someone comes
		// to this page to answer. Computed rather than stored: every input is
		// already here, and a column would only be a slower copy of this that
		// could fall out of step with the UI's own arithmetic.
		//
		// Coarser than the labels on screen. "Short by choice" and "short
		// because the source ran out" are the same query - both are simply not
		// on target - and the finer distinction needs the decision alongside,
		// which the row already carries for display.
		"loudness_outcome": loudnessOutcomeFilter,
		// Why a song is listed, so a page of exceptions can be worked one cause
		// at a time instead of one row at a time.
		"loudness_reason": loudnessReasonFilter,
		"artists_id":      artistFilter,
		"library_id":      libraryIdFilter,
		"path":            containsFilter("media_file.path"),
	}
	// Add all album tags as filters
	for tag := range model.TagMappings() {
		if _, exists := filters[string(tag)]; !exists {
			filters[string(tag)] = tagIDFilter
		}
	}
	return filters
})

// leftAsIsVerdict is the UI's name for a track a run declined to change. It is
// derived from the action rather than stored as a verdict, so it needs its own
// handling wherever a verdict is filtered on.
const leftAsIsVerdict = "left_as_is"

// filterStrings normalises whatever a filter value arrives as - one value, or
// several from a multi-select - into a plain list.
func filterStrings(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func mediaFileRecentlyAddedSort() string {
	if conf.Server.RecentlyAddedByModTime {
		return "media_file.updated_at"
	}
	return "media_file.created_at"
}

func (r *mediaFileRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	query := r.newSelect().
		LeftJoin("media_file_loudness on media_file_loudness.media_file_id = media_file.id")
	query = r.withAnnotation(query, "media_file.id")
	query = r.applyLibraryFilter(query)
	return r.count(query, options...)
}

func (r *mediaFileRepository) CountBySuffix(options ...model.QueryOptions) (map[string]int64, error) {
	sel := r.newSelect(options...).
		Columns("lower(suffix) as suffix", "count(*) as count").
		GroupBy("lower(suffix)")
	var res []struct {
		Suffix string
		Count  int64
	}
	err := r.queryAll(sel, &res)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(res))
	for _, c := range res {
		counts[c.Suffix] = c.Count
	}
	return counts, nil
}

func (r *mediaFileRepository) Exists(id string) (bool, error) {
	return r.exists(Eq{"media_file.id": id})
}

func (r *mediaFileRepository) Put(m *model.MediaFile) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	id, err := r.putByMatch(Eq{"path": m.Path, "library_id": m.LibraryID}, m.ID, &dbMediaFile{MediaFile: m})
	if err != nil {
		return err
	}
	m.ID = id
	return r.updateParticipants(m.ID, m.Participants)
}

func (r *mediaFileRepository) UpdateProbeData(id string, data string) error {
	_, err := r.executeSQL(Update(r.tableName).Set("probe_data", data).Where(Eq{"id": id}))
	return err
}

func (r *mediaFileRepository) selectMediaFile(options ...model.QueryOptions) SelectBuilder {
	columns := append([]string{"media_file.*", "library.path as library_path", "library.name as library_name"},
		loudnessAuditSelectColumns()...)
	sql := r.newSelect(options...).Columns(columns...).
		LeftJoin("library on media_file.library_id = library.id").
		LeftJoin("media_file_loudness on media_file_loudness.media_file_id = media_file.id")
	sql = r.withAnnotation(sql, "media_file.id")
	sql = r.withBookmark(sql, "media_file.id")
	return r.applyLibraryFilter(sql)
}

func (r *mediaFileRepository) Get(id string) (*model.MediaFile, error) {
	res, err := r.GetAll(model.QueryOptions{Filters: Eq{"media_file.id": id}})
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, model.ErrNotFound
	}
	return &res[0], nil
}

func (r *mediaFileRepository) GetWithParticipants(id string) (*model.MediaFile, error) {
	m, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	m.Participants, err = r.getParticipants(m)
	return m, err
}

func (r *mediaFileRepository) GetAll(options ...model.QueryOptions) (model.MediaFiles, error) {
	sq := r.selectMediaFile(options...)
	var res dbMediaFiles
	err := r.queryAll(sq, &res, options...)
	if err != nil {
		return nil, err
	}
	return res.toModels(), nil
}

func (r *mediaFileRepository) GetAllByTags(tag model.TagName, values []string, options ...model.QueryOptions) (model.MediaFiles, error) {
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = v
	}
	tagFilter := Expr(
		fmt.Sprintf("exists (select 1 from json_tree(media_file.tags, '$.%s') where key='value' and value in (%s))",
			tag, strings.Join(placeholders, ",")),
		args...,
	)

	var opts model.QueryOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.Filters != nil {
		opts.Filters = And{tagFilter, opts.Filters}
	} else {
		opts.Filters = tagFilter
	}
	return r.GetAll(opts)
}

func (r *mediaFileRepository) GetCursor(options ...model.QueryOptions) (model.MediaFileCursor, error) {
	sq := r.selectMediaFile(options...)
	cursor, err := queryWithStableResults[dbMediaFile](r.sqlRepository, sq)
	if err != nil {
		return nil, err
	}
	return wrapMediaFileCursor(cursor), nil
}

// FindByPaths finds media files by their paths.
// The paths can be library-qualified (format: "libraryID:path") or unqualified ("path").
// Library-qualified paths search within the specified library, while unqualified paths
// search across all libraries for backward compatibility.
func (r *mediaFileRepository) FindByPaths(paths []string) (model.MediaFiles, error) {
	query := Or{}

	for _, path := range paths {
		parts := strings.SplitN(path, ":", 2)
		if len(parts) == 2 {
			// Library-qualified path: "libraryID:path"
			libraryID, err := strconv.Atoi(parts[0])
			if err != nil {
				// Invalid format, skip
				continue
			}
			relativePath := parts[1]
			query = append(query, And{
				Eq{"path collate nocase": relativePath},
				Eq{"library_id": libraryID},
			})
		} else {
			// Unqualified path: search across all libraries
			query = append(query, Eq{"path collate nocase": path})
		}
	}

	if len(query) == 0 {
		return model.MediaFiles{}, nil
	}

	sel := r.newSelect().Columns("*").Where(query)
	var res dbMediaFiles
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}

	return res.toModels(), nil
}

func (r *mediaFileRepository) Delete(id string) error {
	return r.delete(Eq{"id": id})
}

func (r *mediaFileRepository) DeleteAllMissing() (int64, error) {
	user := loggedUser(r.ctx)
	if !user.IsAdmin {
		return 0, rest.ErrPermissionDenied
	}
	del := Delete(r.tableName).Where(Eq{"missing": true})
	return r.executeSQL(del)
}

func (r *mediaFileRepository) DeleteMissing(ids []string) error {
	user := loggedUser(r.ctx)
	if !user.IsAdmin {
		return rest.ErrPermissionDenied
	}
	return r.delete(
		And{
			Eq{"missing": true},
			Eq{"id": ids},
		},
	)
}

func (r *mediaFileRepository) UpdateComment(ids []string, comment string) error {
	if len(ids) == 0 {
		return nil
	}

	user := loggedUser(r.ctx)
	if !user.IsAdmin {
		return rest.ErrPermissionDenied
	}

	idSeq := slice.SeqFunc(ids, func(id string) string { return id })
	for chunk := range slice.CollectChunks(idSeq, 200) {
		mediaFiles, err := r.GetAll(model.QueryOptions{Filters: Eq{"media_file.id": chunk}})
		if err != nil {
			return err
		}
		for _, mf := range mediaFiles {
			if mf.Path == "" || mf.LibraryPath == "" {
				log.Warn(r.ctx, "Skipping comment write for mediafile without path", "id", mf.ID, "path", mf.Path, "libraryPath", mf.LibraryPath)
				continue
			}
			absPath := mf.AbsolutePath()
			if err := writeMediaFileComment(absPath, comment); err != nil {
				log.Error(r.ctx, "Error updating mediafile comment tag", "id", mf.ID, "path", absPath, err)
				return err
			}
		}

		upd := Update(r.tableName).
			Set("comment", comment).
			Set("updated_at", time.Now()).
			Where(Eq{"id": chunk})
		if _, err := r.executeSQL(upd); err != nil {
			log.Error(r.ctx, "Error updating mediafile comments", "ids", chunk, err)
			return err
		}
		log.Debug(r.ctx, "Updated mediafile comments", "total", len(chunk), "ids", chunk)
	}

	return nil
}

func (r *mediaFileRepository) UpdateLoudnessTags(id string, lufs float64) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}

	mf, err := r.Get(id)
	if err != nil {
		return err
	}
	if mf.Tags == nil {
		mf.Tags = model.Tags{}
	}
	mf.Tags[model.TagName("loudnorm_final_lufs")] = []string{fmt.Sprintf("%.2f", lufs)}

	upd := Update(r.tableName).
		Set("tags", marshalTags(mf.Tags)).
		Set("updated_at", time.Now()).
		Where(Eq{"id": id})
	_, err = r.executeSQL(upd)
	return err
}

// UpdateAudioProperties brings a song's stored description back in step with the
// file on disk.
//
// Only the restore path needs this. Everywhere else the scanner owns these
// columns, and it notices a changed file by its timestamp - but a restore
// replaces the audio without anything asking the scanner to look again, so the
// row goes on describing a file that is no longer there until some later scan
// happens to catch it.
//
// updated_at is deliberately not touched: it is the scanner's own record of when
// it last read the file, and moving it forward here would tell the next scan the
// row is newer than the file it describes.
func (r *mediaFileRepository) UpdateAudioProperties(id string, props model.AudioFileProperties) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	upd := Update(r.tableName).
		Set("bit_rate", props.BitRate).
		Set("sample_rate", props.SampleRate).
		Set("bit_depth", props.BitDepth).
		Set("channels", props.Channels).
		Set("duration", props.Duration).
		Set("size", props.Size).
		Where(Eq{"id": id})
	_, err := r.executeSQL(upd)
	return err
}

func (r *mediaFileRepository) UpdateMissingMetadata(id string, album *string, year *int, genre *string, mbzRecordingID *string, mbzReleaseID *string) error {
	if album == nil && year == nil && genre == nil && mbzRecordingID == nil && mbzReleaseID == nil {
		return nil
	}

	up := Update(r.tableName).Where(Eq{"id": id})

	if album != nil {
		up = up.Set("album", Expr("case when trim(ifnull(album, '')) = '' or lower(trim(ifnull(album, ''))) in ('unknown album', '[unknown album]') then ? else album end", *album))
	}
	if year != nil {
		up = up.Set("year", Expr("case when ifnull(year, 0) = 0 then ? else year end", *year))
	}
	if genre != nil {
		up = up.Set("genre", Expr("case when trim(ifnull(genre, '')) = '' then ? else genre end", *genre))
	}
	if mbzRecordingID != nil {
		up = up.Set("mbz_recording_id", Expr("case when trim(ifnull(mbz_recording_id, '')) = '' then ? else mbz_recording_id end", *mbzRecordingID))
	}
	if mbzReleaseID != nil {
		up = up.Set("mbz_release_id", Expr("case when trim(ifnull(mbz_release_id, '')) = '' then ? else mbz_release_id end", *mbzReleaseID))
	}

	up = up.Set("updated_at", time.Now())
	_, err := r.executeSQL(up)
	return err
}

func (r *mediaFileRepository) UpdateCoverPath(id string, coverPath string) error {
	coverPath = strings.TrimSpace(coverPath)
	if id == "" || coverPath == "" {
		return nil
	}

	up := Update(r.tableName).
		Set("cover_path", coverPath).
		Set("updated_at", time.Now()).
		Where(Eq{"id": id})
	_, err := r.executeSQL(up)
	return err
}

func (r *mediaFileRepository) UpdateSpotifyMetadata(id string, confidence *float64, match *string, artist *string, spotifyURL *string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if confidence == nil && match == nil && artist == nil && spotifyURL == nil {
		return nil
	}

	up := Update(r.tableName).Where(Eq{"id": id})
	if confidence != nil {
		up = up.Set("spotify_confidence", *confidence)
	}
	if match != nil {
		up = up.Set("spotify_match", strings.TrimSpace(*match))
	}
	if artist != nil {
		up = up.Set("spotify_artist", strings.TrimSpace(*artist))
	}
	if spotifyURL != nil {
		up = up.Set("spotify_url", strings.TrimSpace(*spotifyURL))
	}
	up = up.Set("updated_at", time.Now())
	_, err := r.executeSQL(up)
	return err
}

func (r *mediaFileRepository) MarkMissing(missing bool, mfs ...*model.MediaFile) error {
	ids := slice.SeqFunc(mfs, func(m *model.MediaFile) string { return m.ID })
	for chunk := range slice.CollectChunks(ids, 200) {
		upd := Update(r.tableName).
			Set("missing", missing).
			Set("updated_at", time.Now()).
			Where(Eq{"id": chunk})
		c, err := r.executeSQL(upd)
		if err != nil || c == 0 {
			log.Error(r.ctx, "Error setting mediafile missing flag", "ids", chunk, err)
			return err
		}
		log.Debug(r.ctx, "Marked missing mediafiles", "total", c, "ids", chunk)
	}
	return nil
}

func (r *mediaFileRepository) MarkMissingByFolder(missing bool, folderIDs ...string) error {
	for chunk := range slices.Chunk(folderIDs, 200) {
		upd := Update(r.tableName).
			Set("missing", missing).
			Set("updated_at", time.Now()).
			Where(And{
				Eq{"folder_id": chunk},
				Eq{"missing": !missing},
			})
		c, err := r.executeSQL(upd)
		if err != nil {
			log.Error(r.ctx, "Error setting mediafile missing flag", "folderIDs", chunk, err)
			return err
		}
		log.Debug(r.ctx, "Marked missing mediafiles from missing folders", "total", c, "folders", chunk)
	}
	return nil
}

// GetMissingAndMatching returns all mediafiles that are missing and their potential matches (comparing PIDs)
// that were added/updated after the last scan started. The result is ordered by PID.
// It does not need to load bookmarks, annotations and participants, as they are not used by the scanner.
func (r *mediaFileRepository) GetMissingAndMatching(libId int) (model.MediaFileCursor, error) {
	subQ := r.newSelect().Columns("pid").
		Where(And{
			Eq{"media_file.missing": true},
			Eq{"library_id": libId},
		})
	subQText, subQArgs, err := subQ.PlaceholderFormat(Question).ToSql()
	if err != nil {
		return nil, err
	}
	sel := r.newSelect().Columns("media_file.*", "library.path as library_path", "library.name as library_name").
		LeftJoin("library on media_file.library_id = library.id").
		Where("pid in ("+subQText+")", subQArgs...).
		Where(Or{
			Eq{"missing": true},
			ConcatExpr("media_file.created_at > library.last_scan_started_at"),
		}).
		OrderBy("pid")
	cursor, err := queryWithStableResults[dbMediaFile](r.sqlRepository, sel)
	if err != nil {
		return nil, err
	}
	return wrapMediaFileCursor(cursor), nil
}

func wrapMediaFileCursor(cursor iter.Seq2[dbMediaFile, error]) model.MediaFileCursor {
	return func(yield func(model.MediaFile, error) bool) {
		for m, err := range cursor {
			if m.MediaFile == nil {
				yield(model.MediaFile{}, fmt.Errorf("unexpected nil mediafile (%v): %w", m, err))
				return
			}
			if !yield(*m.MediaFile, err) || err != nil {
				return
			}
		}
	}
}

// FindRecentFilesByMBZTrackID finds recently added files by MusicBrainz Track ID in other libraries
// It uses a lightweight query without annotation/bookmark joins since those are not needed for matching
func (r *mediaFileRepository) FindRecentFilesByMBZTrackID(missing model.MediaFile, since time.Time) (model.MediaFiles, error) {
	sel := r.newSelect().Columns("media_file.*", "library.path as library_path", "library.name as library_name").
		LeftJoin("library on media_file.library_id = library.id").
		Where(And{
			NotEq{"media_file.library_id": missing.LibraryID},
			Eq{"media_file.mbz_release_track_id": missing.MbzReleaseTrackID},
			NotEq{"media_file.mbz_release_track_id": ""}, // Exclude empty MBZ Track IDs
			Eq{"media_file.suffix": missing.Suffix},
			Gt{"media_file.created_at": since},
			Eq{"media_file.missing": false},
		}).OrderBy("media_file.created_at DESC")

	var res dbMediaFiles
	err := r.queryAll(sel, &res)
	if err != nil {
		return nil, err
	}
	return res.toModels(), nil
}

// FindRecentFilesByProperties finds recently added files by intrinsic properties in other libraries
// It uses a lightweight query without annotation/bookmark joins since those are not needed for matching
func (r *mediaFileRepository) FindRecentFilesByProperties(missing model.MediaFile, since time.Time) (model.MediaFiles, error) {
	sel := r.newSelect().Columns("media_file.*", "library.path as library_path", "library.name as library_name").
		LeftJoin("library on media_file.library_id = library.id").
		Where(And{
			NotEq{"media_file.library_id": missing.LibraryID},
			Eq{"media_file.title": missing.Title},
			Eq{"media_file.size": missing.Size},
			Eq{"media_file.suffix": missing.Suffix},
			Eq{"media_file.disc_number": missing.DiscNumber},
			Eq{"media_file.track_number": missing.TrackNumber},
			Eq{"media_file.album": missing.Album},
			Eq{"media_file.mbz_release_track_id": ""}, // Exclude files with MBZ Track ID
			Gt{"media_file.created_at": since},
			Eq{"media_file.missing": false},
		}).OrderBy("media_file.created_at DESC")

	var res dbMediaFiles
	err := r.queryAll(sel, &res)
	if err != nil {
		return nil, err
	}
	return res.toModels(), nil
}

var mediaFileSearchConfig = searchConfig{
	NaturalOrder: "media_file.rowid",
	OrderBy:      []string{"title"},
	MBIDFields:   []string{"mbz_recording_id", "mbz_release_track_id"},
}

func (r *mediaFileRepository) Search(q string, options ...model.QueryOptions) (model.MediaFiles, error) {
	var opts model.QueryOptions
	if len(options) > 0 {
		opts = options[0]
	}
	var res dbMediaFiles
	err := r.doSearch(r.selectMediaFile(options...), q, &res, mediaFileSearchConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("searching media_file %q: %w", q, err)
	}
	return res.toModels(), nil
}

func (r *mediaFileRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(r.ctx, options...))
}

func (r *mediaFileRepository) Read(id string) (any, error) {
	return r.Get(id)
}

func (r *mediaFileRepository) ReadAll(options ...rest.QueryOptions) (any, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *mediaFileRepository) EntityName() string {
	return "mediafile"
}

func (r *mediaFileRepository) NewInstance() any {
	return &model.MediaFile{}
}

var _ model.MediaFileRepository = (*mediaFileRepository)(nil)
var _ model.ResourceRepository = (*mediaFileRepository)(nil)
