# 本家Mozzi(Arduino C++版)を理解する

このドキュメントは、移植元である本家Mozzi([sensorium/Mozzi](https://github.com/sensorium/Mozzi)、[公式ドキュメント](https://sensorium.github.io/Mozzi/))のアーキテクチャと主要概念を解説します。TinyGo版のコードを読む前に、まず「元々何を移植しようとしているのか」を掴むためのドキュメントです。

本家のソースはこのリポジトリの[`../../Mozzi`](../../Mozzi)にそのまま入っています。以下で参照するファイルパスは、すべてそのフォルダ基準です。

## 1. Mozziとは何か

Mozziは、Arduino(やその互換ボード)のようなOSを持たない小型マイコン上で、**リアルタイムに音を合成する**ためのC++ライブラリです。普通のArduinoスケッチが`setup()`/`loop()`しか持たないのに対し、Mozziは「音声波形を1サンプルずつ、正確なタイミングで生成し続ける」という、シビアなリアルタイム性が要求される処理を裏で行います。

用途としては、シンセサイザーやゲーム機の効果音、電子楽器の自作(チップチューン、モジュラーシンセの自作モジュールなど)が中心です。

## 2. 実行モデル: 2つの「レート」

Mozziスケッチは、Arduinoの`setup()`/`loop()`の中に、さらに3つの関数を書きます。

```cpp
void setup() {
  startMozzi();          // Mozziのタイマー割り込み等を初期化して開始
}

void updateControl() {
  // 「遅い」処理: センサー読み取り、エンベロープの進行、周波数の変更など
  // 呼ばれる頻度 = MOZZI_CONTROL_RATE (デフォルト64Hz)
}

AudioOutput updateAudio() {
  // 「速い」処理: 1サンプル分の波形を計算して返す
  // 呼ばれる頻度 = MOZZI_AUDIO_RATE (デフォルト16384Hz)
  return MonoOutput::from8Bit(...);
}

void loop() {
  audioHook();  // 必須: これがMozziの心臓部を回す
}
```

なぜ2つのレートに分けるのか: 音声波形の生成(`updateAudio`)は16384回/秒のような高頻度で正確なタイミングが必要ですが、「今どの周波数を鳴らすか」「エンベロープが今どのフェーズか」のような制御ロジックはそこまで頻繁に計算し直す必要がありません。制御ロジックを遅いレートに分離することで、CPU負荷を大きく削減できます。これがMozziの設計の核心です。

## 3. `audioHook()`とバッファリングの仕組み

`audioHook()`(実体は[`internal/MozziGuts.hpp`](../../Mozzi/internal/MozziGuts.hpp))は、`loop()`の中で「できるだけ速く」何度も呼ばれます。中身はおおよそ以下の流れです。

1. 出力用の[`CircularBuffer`](../../Mozzi/CircularBuffer.h)(リングバッファ)に空きがあるかチェック
2. 空きがあれば:
   - 一定間隔ごとに`updateControl()`を1回呼ぶ(間隔は`MOZZI_AUDIO_RATE / MOZZI_CONTROL_RATE`から計算)
   - 毎回`updateAudio()`を1回呼び、その結果をバッファに書き込む
3. **これとは別に**、プラットフォーム固有のハードウェアタイマー割り込みが`MOZZI_AUDIO_RATE`の正確な周期で発火し、バッファから1サンプル取り出してDAC/PWM等に出力する

つまり「音声データを生成する速度(`loop()`がどれだけ速く回るか、コード次第で変動しうる)」と「音声データを出力する速度(ハードウェアタイマーで固定・正確)」を、リングバッファで**分離**しています。これにより、`updateAudio()`の中身が多少重くても(バッファに余裕がある限り)出力のタイミングが乱れません。

`CircularBuffer`自体も一工夫あります。単純な「読み込み位置==書き込み位置」だけでは「空」か「満杯」かを区別できないため、位置が一周したかどうかを示す1ビット(`s_msb`/`e_msb`)を追加で持たせる、という古典的なテクニックが使われています([`CircularBuffer.h`](../../Mozzi/CircularBuffer.h))。TinyGo版の`engine.ringBuffer`もこの仕組みをそのまま移植しています。

## 4. 固定小数点演算(`mozzi_fixmath.h`)

多くのマイコン(特にAVR)には浮動小数点演算ユニット(FPU)がなく、`float`の計算はソフトウェアエミュレーションで非常に遅くなります。そのためMozziの内部処理は基本的に**整数の固定小数点演算**で行われます。

[`mozzi_fixmath.h`](../../Mozzi/mozzi_fixmath.h)では`QmnN`という命名規則で型が定義されています。

- `Qmn` の `m` = 整数部のビット数、`n` = 小数部のビット数
- 例: `Q15n16`は`int32_t`で、上位15ビットが整数部・下位16ビットが小数部を表す符号付き固定小数点数。`1.0`は`1 << 16 = 65536`として表現される
- 例: `Q8n8`は`uint16_t`で、8ビット整数部+8ビット小数部

変換は「浮動小数点の値に`2^n`を掛けて整数に丸める」だけです。これにより、乗算・除算がすべて整数のシフト演算と乗算で完結し、非常に高速に動きます。

TinyGo版では[`fixmath`](../fixmath)パッケージがこれをそのまま移植しています(型名・関数名も可能な限り対応させています)。

## 5. 波形テーブルとオシレータ(`Oscil.h`)

Mozziの音源の基本は**波形テーブル(wavetable)方式**です。1周期分の波形(サイン波、ノコギリ波など)を配列(`int8_t`の配列、-128〜127)として持っておき、それを一定speedで読み進めることで任意の周波数の音を作ります。

[`Oscil.h`](../../Mozzi/Oscil.h)の心臓部は「位相アキュムレータ」です。

- `phase_fractional`(32ビット整数)が現在の再生位置を、16ビット分の小数精度付きで保持
- 毎サンプル`phase_increment_fractional`を加算する(オーバーフローで自然にループする)
- テーブルの実際のインデックスは`phase_fractional >> 16`をテーブルサイズでマスクして求める
- `phase_increment_fractional`の値を変えるだけで周波数が変わる(`setFreq()`はこの値を計算しているだけ)

このテクニックのおかげで、除算や三角関数を毎サンプル計算する必要がなく、整数の加算だけで正確な周波数のオシレータが実現できます。TinyGo版の[`oscil.Oscil`](../oscil/oscil.go)もこのロジックをそのまま踏襲しています。

テーブル自体は[`tables/`](../../Mozzi/tables)フォルダに`.h`ファイルとして大量に用意されています(`CONSTTABLE_STORAGE`マクロでPROGMEM=フラッシュメモリに配置)。AVRはハーバードアーキテクチャで、フラッシュ上のデータをそのまま読めず`pgm_read_byte()`のような専用命令が必要なため、`FLASH_OR_RAM_READ`というマクロで抽象化されています(詳細は[`mozzi_pgmspace.h`](../../Mozzi/mozzi_pgmspace.h))。**この制約はRP2040/ESP32のような32ビットマイコンには存在しない**ため、TinyGo版ではこの部分がむしろシンプルになっています(詳細は[02-port-architecture.md](02-port-architecture.md)参照)。

## 6. その他の主要ビルディングブロック

| ファイル | 役割 |
|---|---|
| [`Line.h`](../../Mozzi/Line.h) | 2点間を一定ステップで線形補間する(周波数のグライドなどに使用) |
| [`ADSR.h`](../../Mozzi/ADSR.h) | Attack/Decay/Sustain/Releaseのエンベロープ。`update()`(制御レート)と`next()`(補間、通常オーディオレート)を分離しているのが特徴 |
| [`Ead.h`](../../Mozzi/Ead.h) | 指数的なアタック/ディケイのエンベロープ(打楽器的な音に向く) |
| [`Smooth.h`](../../Mozzi/Smooth.h) | 単純な1次IIRローパスフィルタ。制御信号のノイズ除去・平滑化に使う |
| [`EventDelay.h`](../../Mozzi/EventDelay.h) | ノンブロッキングの`delay()`代替。`audioTicks()`(出力済みサンプル数)を基準に時間を測る |
| [`AudioDelay.h`](../../Mozzi/AudioDelay.h) / [`AudioDelayFeedback.h`](../../Mozzi/AudioDelayFeedback.h) | リングバッファ式のディレイライン(コムフィルタ、フランジャー、エコーなど) |
| [`ResonantFilter.h`](../../Mozzi/ResonantFilter.h) | Paul Kellett設計の固定小数点レゾナンスフィルタ(LPF/HPF/BPF/ノッチ) |
| [`mozzi_rand.h`](../../Mozzi/mozzi_rand.h) | xorshift96による高速疑似乱数(Arduinoの`random()`は音声生成に使うには遅すぎる) |
| [`mozzi_midi.h`](../../Mozzi/mozzi_midi.h) | MIDIノート番号→周波数変換(ルックアップテーブル+補間) |
| [`AudioOutput.h`](../../Mozzi/AudioOutput.h) | `MonoOutput`/`StereoOutput`。「何ビット幅の値を渡したか」を`fromNBit()`等で明示することで、出力ビット幅が違うプラットフォーム間でもスケッチのコードを共通化できる仕組み |

## 7. プラットフォーム(ハードウェア)の抽象化

Mozziは非常に多くのマイコンに対応しています(AVR, ESP32, ESP8266, SAMD21, RP2040, STM32系複数種, Teensy, MBED, Renesas, CH32など)。この差異は[`internal/MozziGuts_impl_<PLATFORM>.hpp`](../../Mozzi/internal)というファイル群に隔離されており、共通部分の[`internal/MozziGuts.hpp`](../../Mozzi/internal/MozziGuts.hpp)から`#include`で切り替えられます。

各プラットフォーム実装が提供する必要があるものは、大きく以下の3つです。

1. **`startAudio()`**: 音声出力用ハードウェア(PWM/DAC/I2S)とタイマー割り込みの初期化
2. **`audioOutput(sample)`**: 1サンプルをハードウェアに書き込む
3. (アナログ入力を使う場合)**ADC関連の関数群**

出力方式自体も`MOZZI_AUDIO_MODE`という設定で複数選べます(`MOZZI_OUTPUT_PWM`、`MOZZI_OUTPUT_I2S_DAC`、`MOZZI_OUTPUT_EXTERNAL_TIMED`など)。RP2040の実装例は[`internal/MozziGuts_impl_RP2040.hpp`](../../Mozzi/internal/MozziGuts_impl_RP2040.hpp)にあり、TinyGo版のバックエンド設計はここを主に参考にしています(ただし本家はArduino-picoコアの`PWMAudio`ライブラリに依存しており、TinyGo版はそれを使わず一から実装しています。詳細は[04-hardware-backends.md](04-hardware-backends.md))。

設定値自体は[`MozziConfigValues.h`](../../Mozzi/MozziConfigValues.h)で定義され、プラットフォームごとの設定妥当性チェックが[`internal/config_checks_*.h`](../../Mozzi/internal)群にあります。

## 8. サンプルの構成

[`examples/`](../../Mozzi/examples)配下に100以上のサンプルスケッチが、番号付きのカテゴリフォルダに分類されています(`01.Basics`, `02.Control`, `06.Synthesis`, `07.Envelopes`, `09.Delays`, `10.Audio_Filters`...)。TinyGo版のサンプルは、このうちごく一部を移植したものです(対応関係は[03-feature-coverage.md](03-feature-coverage.md)参照)。

## 9. 用語集

| 用語 | 意味 |
|---|---|
| **CONTROL_RATE** | `updateControl()`が呼ばれる頻度。デフォルト64Hz |
| **AUDIO_RATE** | `updateAudio()`が呼ばれ、音声サンプルが出力される頻度。デフォルト16384Hz |
| **ウェーブテーブル(wavetable)** | 1周期分の波形を格納した配列。再生速度を変えることで音程を変える方式 |
| **位相アキュムレータ(phase accumulator)** | ウェーブテーブル中の現在位置を、固定小数点の値として保持・加算していく仕組み |
| **Q形式(Qmn記法)** | 固定小数点数の表記法。`m`ビット整数部+`n`ビット小数部 |
| **PROGMEM** | AVRでフラッシュメモリ上に定数データを配置する仕組み。専用の読み出し命令が必要 |
| **ISR (Interrupt Service Routine)** | 割り込みハンドラ。Mozziでは正確なタイミングでの音声出力に使われる |
| **エンベロープ(envelope)** | 音量や音色が時間とともにどう変化するかを表す制御信号(ADSRなど) |
| **エイリアシング(aliasing)** | サンプリング周波数の半分を超える周波数成分が折り返して現れる現象。高い音を鳴らす際にMozziでも問題になりうる |

## 10. さらに詳しく知りたい場合

- 公式ドキュメント: https://sensorium.github.io/Mozzi/
- 本家GitHub: https://github.com/sensorium/Mozzi
- リポジトリ内: [`../../Mozzi/README.md`](../../Mozzi/README.md), [`../../Mozzi/Readme_Mozzi_2_0.md`](../../Mozzi/Readme_Mozzi_2_0.md)
- 各クラスのAPIドキュメントは、対応する`.h`ファイルの冒頭コメント(Doxygen形式)に書かれています
