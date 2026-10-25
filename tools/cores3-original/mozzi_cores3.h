// mozzi_cores3.h -- lets an unmodified upstream Mozzi example sketch play
// through an M5Stack CoreS3's built-in speaker, for side-by-side comparison
// with this port's examples (built with -tags cores3). build.sh includes it at the
// very top of a copy of the sketch, and calls cores3EnableSpeaker() at the
// start of its setup().
//
// It configures Mozzi the way the TinyGo port runs on this board: I2S out
// of BCK=G34 / WS=G33 / DATA=G13 at a 16384Hz audio rate (upstream's ESP32
// default is 32768Hz), and wakes the AW88298 amp up over I2C with exactly
// the register writes mozzi/board/cores3 does.
#pragma once

#include <Arduino.h>
#include <Wire.h>

#include "MozziConfigValues.h"

#define MOZZI_AUDIO_MODE MOZZI_OUTPUT_I2S_DAC
#define MOZZI_AUDIO_RATE 16384
#define MOZZI_I2S_PIN_BCK 34
#define MOZZI_I2S_PIN_WS 33
#define MOZZI_I2S_PIN_DATA 13

static void cores3WriteReg8(uint8_t addr, uint8_t reg, uint8_t value) {
  Wire.beginTransmission(addr);
  Wire.write(reg);
  Wire.write(value);
  Wire.endTransmission();
}

static uint8_t cores3ReadReg8(uint8_t addr, uint8_t reg) {
  Wire.beginTransmission(addr);
  Wire.write(reg);
  Wire.endTransmission(false);
  Wire.requestFrom(addr, (uint8_t)1);
  return Wire.available() ? Wire.read() : 0;
}

static void cores3UpdateReg8(uint8_t addr, uint8_t reg, uint8_t set, uint8_t clear) {
  cores3WriteReg8(addr, reg, (cores3ReadReg8(addr, reg) & ~clear) | set);
}

static void cores3WriteAW88298(uint8_t reg, uint16_t value) {
  Wire.beginTransmission(0x36);
  Wire.write(reg);
  Wire.write(value >> 8);
  Wire.write(value & 0xFF);
  Wire.endTransmission();
}

static void cores3EnableSpeaker() {
  Wire.begin(12, 11, 400000); // internal I2C: SDA=G12, SCL=G11

  // AW9523B P0_2 (AW88298 reset): drive it high as a push-pull GPIO output.
  const uint8_t aw9523 = 0x58, resetBit = 1 << 2;
  cores3UpdateReg8(aw9523, 0x02, resetBit, 0); // output P0
  cores3UpdateReg8(aw9523, 0x12, resetBit, 0); // LED mode P0: GPIO
  cores3UpdateReg8(aw9523, 0x11, 1 << 4, 0);   // GCR: P0 push-pull
  cores3UpdateReg8(aw9523, 0x04, 0, resetBit); // config P0: output

  // Sample rate field of AW88298 register 0x06, chosen like M5Unified does.
  static const uint8_t rateTable[] = {4, 5, 6, 8, 10, 11, 15, 20, 22, 44};
  uint16_t rate = (MOZZI_AUDIO_RATE + 1102) / 2205, code = 0;
  while (code < sizeof(rateTable) - 1 && rate > rateTable[code]) code++;

  cores3WriteAW88298(0x61, 0x0673);        // boost mode disabled
  cores3WriteAW88298(0x04, 0x4040);        // I2SEN=1 AMPPD=0 PWDN=0
  cores3WriteAW88298(0x05, 0x0008);        // RMSE=0 HAGCE=0 HDCCE=0 HMUTE=0
  cores3WriteAW88298(0x06, 0x34C0 | code); // I2S, BCK mode 16*2, -6dB input (INPLEV), sample rate: same as mozzi/board/cores3
  cores3WriteAW88298(0x0C, 0x2064);        // volume -12dB: same as mozzi/board/cores3 (see its amp level notes)
}
