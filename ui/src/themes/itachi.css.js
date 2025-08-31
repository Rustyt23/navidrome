const stylesheet = `
.react-jinke-music-player-main svg:active, .react-jinke-music-player-main svg:hover {
    color: #e53935
}

.react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-handle, .react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-track {
    background-color: #e53935
}

.react-jinke-music-player-main ::-webkit-scrollbar-thumb {
    background-color: #e53935;
}

.react-jinke-music-player-main .music-player-panel .panel-content .rc-slider-handle:active {
    box-shadow: 0 0 2px #e53935
}

.react-jinke-music-player-main .audio-item.playing svg {
    color: #e53935
}

.react-jinke-music-player-main .audio-item.playing .player-singer {
    color: #e53935 !important
}

.audio-lists-panel-content .audio-item.playing, .audio-lists-panel-content .audio-item.playing svg {
    color: #e53935
}

.audio-lists-panel-content .audio-item:active .group:not([class=".player-delete"]) svg, .audio-lists-panel-content .audio-item:hover .group:not([class=".player-delete"]) svg {
    color: #e53935
}
`

export default stylesheet
