# Normalize bundled schemas for OpenAI strict JSON:
# 1) Rewrite "$ref": "./foo.json" to "$ref": "#/$defs/<matching-full-URL>"
# 2) Rekey $defs from URL-like keys to simple basenames (strip scheme, path, and .json)
# 3) Retarget any "$ref": "#/$defs/<urlish>" to the rekeyed basename

reduce (paths(scalars) | select(.[-1] == "$ref")) as $p
  (.;
    (getpath($p)) as $ref |
    if ($ref | type) == "string" and ($ref | startswith("./")) and ($ref | endswith(".json")) then
      ($ref | sub("^\\./"; "")) as $fname |
      ((.["$defs"] // {} | keys | map(select(endswith($fname))) | .[0])) as $url |
      if $url then setpath($p; "#/$defs/" + $url) else . end
    else
      .
    end
  )
|
( if has("$defs") then
    (. ["$defs"] | keys) as $defkeys
    | reduce $defkeys[] as $k
        (.;
          ( ($k
              | (if ($k | test("^https?://")) then (sub("^https?://"; "")) else . end)
              | split("/")
              | last
              | sub("\\.json$"; "")
             ) as $base
          | if $base == $k then . else
              .["$defs"][$base] = .["$defs"][$k]
              | del(.["$defs"][$k])
            end
          )
        )
  else . end )
|
reduce (paths(scalars) | select(.[-1] == "$ref")) as $p
  (.;
    (getpath($p)) as $ref |
    if ($ref | type) == "string" and ($ref | startswith("#/$defs/")) then
      ($ref | sub("^#/\\$defs/"; "")) as $tail |
      ( ($tail
          | (if test("^https?://") then (sub("^https?://"; "")) else . end)
          | split("/")
          | last
          | sub("\\.json$"; "")
        ) as $name
      | setpath($p; "#/$defs/" + $name))
    else
      .
    end
  )
