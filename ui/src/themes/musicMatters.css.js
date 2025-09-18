const stylesheet = `
.react-jinke-music-player-main svg:active, .react-jinke-music-player-main svg:hover {
    color: #FF2B8A;
}

.react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-handle,
.react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-track {
    background-color: #FF2B8A;
    border-color: #E11D74;
}

.react-jinke-music-player-main ::-webkit-scrollbar-thumb {
    background-color: #FF2B8A;
}

.react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-handle:active {
    box-shadow: 0 0 2px #FF2B8A;
}

.react-jinke-music-player-main .audio-item.playing svg {
    color: #FF2B8A;
}

.react-jinke-music-player-main .audio-item.playing .player-singer {
    color: #FF2B8A !important;
}

.audio-lists-panel-content .audio-item.playing,
.audio-lists-panel-content .audio-item.playing svg {
    color: #FF2B8A;
}
.audio-lists-panel-content .audio-item:active .group:not([class=".player-delete"]) svg,
.audio-lists-panel-content .audio-item:hover .group:not([class=".player-delete"]) svg {
    color: #FF2B8A;
}
`

export default stylesheet
