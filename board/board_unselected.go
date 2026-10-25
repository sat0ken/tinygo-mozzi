//go:build (esp32s3 && !cores3 && !esp32s3pwm && !tinygoconf2026badge) || (!rp2040 && !esp32s3)

package board

// No board selected: an ESP32-S3 build needs -tags cores3 (M5Stack CoreS3),
// -tags tinygoconf2026badge (TinyGo Conf 2026 badge) or -tags esp32s3pwm
// (PWM out of GPIO4), and other targets aren't supported. Referencing this undefined name makes the build fail with a
// pointer to the fix instead of silently driving the wrong pins.
var _ = boardNotSelected_buildWith_tags_cores3_tinygoconf2026badge_or_esp32s3pwm_see_mozzi_board_package_doc

const defaultVolume = 255

func applyVolume(uint8) {}
