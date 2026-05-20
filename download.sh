#!/usr/bin/env bash
yt-dlp -f "bestaudio/best" \
  --ignore-errors \
  --sleep-interval 5 --max-sleep-interval 15 \
  --write-subs --sub-langs "all,-live_chat" --convert-subs srt --embed-subs \
  --embed-metadata --embed-thumbnail --convert-thumbnails jpg \
  --remux-video mka \
  --download-archive archive.txt \
  -o "%(title).100B [%(uploader).50B] [%(id)s].%(ext)s" \
 "https://music.youtube.com/watch?v=Soy4jGPHr3g"
