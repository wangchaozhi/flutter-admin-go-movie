# media_kit (vendored fork)

This is a vendored copy of [`media_kit` 1.2.6](https://pub.dev/packages/media_kit)
wired in via `dependency_overrides` in `front/mobile/pubspec.yaml`.

## Why fork?

Upstream media_kit plays HLS on the web through **hls.js**, but its web
`Player.setAudioTrack(...)` implementation does not drive hls.js — it only
appends a `<source>` element to the `<video>` tag, which the browser ignores
once media is already loaded. As a result, switching the audio track on web is
a silent no-op (no network request, no audio change). Native (libmpv) is not
affected.

## What was patched

Only two files differ from upstream 1.2.6:

- `lib/src/player/web/utils/hls.dart`
  - Exposed `Hls.audioTracks` (getter), `Hls.audioTrack` (getter/setter) on the
    JS interop binding.

- `lib/src/player/web/player/real.dart`
  - Keep a reference to the active `Hls` instance (`_hls`) created in
    `_loadSource`, and clear it for non-HLS media.
  - In `setAudioTrack`, when an `Hls` instance is active and the requested
    `AudioTrack.id` is `auto` or a numeric rendition index, switch via
    `hls.audioTrack = index` instead of the `<source>` hack.

The numeric index matches the order of `#EXT-X-MEDIA:TYPE=AUDIO` renditions in
the master playlist, which the backend emits in `stream_position` order
(`backend/internal/video/hls_handler.go`). `id == '0'` / `auto` selects the
default (muxed) rendition.

Keep this in sync if media_kit is upgraded.
