# 機能対応表: 何を移植し、何を移植していないか

本家Mozzi(`Mozzi/`配下)の主要ファイルを基準に、TinyGo版での対応状況を一覧化したものです。「✅ 移植済み」「⚠️ 部分的」「❌ 未移植」で分類しています。

## コア・スケジューラ

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `internal/MozziGuts.hpp` | ✅ | [`engine`](../engine) | `Sketch`/`Backend`インターフェース、`AudioHook()`、`stopMozzi()`相当の`engine.Stop()`(本家ESP32版は未実装) |
| `CircularBuffer.h` | ✅ | [`engine/ringbuffer.go`](../engine/ringbuffer.go) | 256要素固定でmirroring bit方式を移植 |
| `AudioOutput.h` | ✅ | [`audioout`](../audioout) | `MonoOutput`/`StereoOutput`。ただしビット幅は8固定([02](02-port-architecture.md#24-出力ビット幅を8ビット固定に単純化)参照) |
| `internal/MozziGuts_impl_*.hpp`(各プラットフォーム) | ⚠️ | [`backend/*`](../backend) | RP2040/ESP32-S3の4バックエンド(各PWM出力、各I2S出力)のみ。他プラットフォームは未移植 |
| `MozziConfigValues.h` / `internal/config_checks_*.h` | ❌ | — | コンパイル時設定バリデーションの仕組み自体を移植していない(Goでは構成がシンプルなため不要と判断) |
| `mozzi_analog.h` | ⚠️ | [`analog`](../analog) | `mozziAnalogRead`を移植。本家がESP32/RP2040で行うのと同じ同期(ブロッキング)読み取り。AVRの非同期ADCの仕組みは未移植 |

## 数学・ユーティリティ

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `mozzi_fixmath.h` / `.cpp` | ✅ | [`fixmath`](../fixmath) | 旧来のtypedef形式のAPI。新しい`FixMath`(`UFix`/`SFix`)ライブラリは❌ |
| `mozzi_rand.h` / `internal/mozzi_rand_p.h` | ✅ | [`rand`](../rand) | xorshift96、既定シード値も本家と同一 |
| `mozzi_midi.h` | ✅ | [`midi`](../midi) | `mtof`/`Q16n16_mtof`相当。ただし`Q16n16MtoF`は末尾での配列外読み出しをクランプに変更(意図的な安全側の変更) |
| `mult16x8.h`, `mult32x16.h`, `mult16x16.h` | ❌ | — | AVRアセンブラ最適化された乗算関数群。32ビットCPUでは意味がないため未移植 |
| `IntegerType.h` | ⚠️ | — | 「必要な最小のビット幅の型を選ぶ」ためのテンプレートメタプログラミング。Go版では`filter`/`delay`パッケージ内で「常に十分広い型(int32/int64)を使う」方針に置き換え、この仕組み自体は移植していない |
| `Stack.h`, `primes.h`, `meta.h`, `RollingStat.h` | ❌ | — | 主にADC関連や内部実装補助。未使用 |

## オシレータ・波形

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `Oscil.h` | ✅ | [`oscil.Oscil`](../oscil/oscil.go) | `setFreq`系メソッド、`phMod`含めほぼ全機能 |
| `Phasor.h` | ✅ | [`oscil.Phasor`](../oscil/phasor.go) | |
| `tables/*.h`(フラット、59種) | ✅ | [`tables`](../tables) | `tools/gentables`で自動生成 |
| `tables/BandLimited_SAW/SQUARE/TRI` | ❌ | — | 合計約4MBと大きいため既定では未生成。`gentables`で個別に生成可能([../README.md](../README.md#波形テーブル)) |
| `MetaOscil.h` | ❌ | — | 複数テーブルを周波数に応じて自動切替するオシレータ。上のBandLimitedテーブルと組み合わせて使うもので、未移植 |
| `WavePacket.h`, `WavePacketSample.h` | ❌ | — | ウェーブパケット合成 |
| `OverSample.h` | ❌ | — | オーバーサンプリング |
| `SampleHuffman.h`、`samples/thumbpiano_huffman/*.h` | ✅ | [`samplehuffman`](../samplehuffman)、[`samples/thumbpianohuffman`](../samples/thumbpianohuffman) | ハフマン圧縮サンプルの再生。データは`tools/genhuffman`で変換。復号結果は本家と全サンプルでビット一致を確認 |
| `samples/*.h`(その他)、`Sample.h` | ❌ | — | 非圧縮サンプルの再生 |

## 制御信号・エンベロープ

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `Line.h` | ✅ | [`line.Line[T]`](../line/line.go) | ジェネリクスで型ごとの特殊化を1本化 |
| `ADSR.h` | ✅ | [`envelope.ADSR`](../envelope/adsr.go) | |
| `Ead.h` | ✅ | [`envelope.Ead`](../envelope/ead.go) | |
| `Smooth.h` | ✅ | [`envelope.SmoothInt8`/`SmoothUint8`/`SmoothInt32`/`SmoothFloat32`](../envelope/smooth.go) | 本家のテンプレート特殊化ごとに別型として実装(Go版はジェネリクス化していない。理由は同ファイルのコメント参照) |
| `EventDelay.h` | ✅ | [`eventdelay.EventDelay`](../eventdelay/eventdelay.go) | |
| `Portamento.h` | ❌ | — | MIDIノート間のポルタメント補間 |
| `AutoRange.h`, `AutoMap.h`, `IntMap.h` | ❌ | — | センサー値の自動レンジ調整・マッピング |
| `RollingAverage.h` | ✅ | [`rollingaverage`](../rollingaverage) | 移動平均(整数版とfloat版) |
| `RollingStat.h` | ❌ | — | 統計 |
| `DCfilter.h` | ❌ | — | DCオフセット除去フィルタ |
| `CapPoll.h`, `RCpoll.h` | ❌ | — | 静電容量センサ・CR回路によるアナログ入力読み取り |

## フィルタ

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `ResonantFilter.h`(`LowPassFilter.h`は非推奨エイリアス) | ✅ | [`filter.Filter8`/`Filter16`](../filter/filter.go) | LOWPASS/HIGHPASS/BANDPASS/NOTCHを常に全部計算する設計に統合([02](02-port-architecture.md)参照) |
| `StateVariable.h` | ❌ | — | ステートバリアブルフィルタ |
| `PDResonant.h` | ❌ | — | Pure Data由来のレゾナントフィルタ |
| `WaveFolder.h`, `WaveShaper.h` | ❌ | — | 波形折り返し/波形シェイピング歪み系エフェクト |

## ディレイ・空間系エフェクト

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `AudioDelay.h` | ✅ | [`delay.AudioDelay[T]`](../delay/audiodelay.go) | |
| `AudioDelayFeedback.h` | ⚠️ | [`delay.AudioDelayFeedback`](../delay/audiodelayfeedback.go) | LINEAR補間・int8セルのみ。ALLPASS補間モードとint16セル版は未移植 |
| `ControlDelay.h` | ✅ | [`delay.AudioDelay[T]`](../delay/audiodelay.go) | 本家でも`AudioDelay`を制御レートで使うだけのクラスなので、`delay.New[int32](n)`で同じことができる |
| `Metronome.h` | ✅ | [`metronome`](../metronome) | 遅れを持ち越さない再スケジュールも本家どおり |
| `ReverbTank.h` | ❌ | — | リバーブ |

## 通信・その他

| 本家ファイル | 状態 | TinyGo版 | 備考 |
|---|:---:|---|---|
| `twi_nonblock.h` / `internal/twi_nonblock.hpp` | ❌ | — | ノンブロッキングI2C(TWI)通信 |
| MIDI入出力(`examples/11.Communication`) | ❌ | — | シリアル/USB MIDI |
| `hardware_defines.h`, `internal/mozzi_macros.h`, `internal/teensyPinMap.h` | ❌ | — | プラットフォーム判定・ピンマップ関連の内部マクロ |

## 独自追加(本家に存在しない)

| TinyGo版 | 説明 |
|---|---|
| [`song`](../song) | メロディ再生用シーケンサ。`[]song.Event{Note, Beats}`を`song.Player`で再生 |

## サンプルスケッチの対応

| 本家サンプル | TinyGo版サンプル |
|---|---|
| `01.Basics/*`(全8本) | [`examples/01.Basics/*`](../examples/01.Basics) — Sinewave, Sinewave_HIFI, Control_Gain, Skeleton, Skeleton_Multi, Table_Resolution, Vibrato, Vibrato_Midi_Note |
| `02.Control/*`(全8本) | [`examples/02.Control/*`](../examples/02.Control) — Control_Echo_Theremin, Control_Oscil_Wash, Control_Tremelo, EventDelay, Line_Gliss, Line_Gliss_Double_32k_HIFI, Metronome_SampleHuffman, Stop_Start |
| `06.Synthesis/FMsynth` | [`examples/06.Synthesis/FMsynth`](../examples/06.Synthesis/FMsynth) |
| `07.Envelopes/ADSR_Audio_Rate_Envelope` | [`examples/07.Envelopes/ADSR_Audio_Rate_Envelope`](../examples/07.Envelopes/ADSR_Audio_Rate_Envelope) |
| `09.Delays/AudioDelay` | [`examples/09.Delays/AudioDelay`](../examples/09.Delays/AudioDelay) |
| `10.Audio_Filters/ResonantFilter` | [`examples/10.Audio_Filters/ResonantFilter`](../examples/10.Audio_Filters/ResonantFilter) |
| (該当なし、独自) | [`examples/TinyGo_Extras/`](../examples/TinyGo_Extras) — fanfare, furuhata_op(`song`パッケージのデモ)、fanfare_synth(手書きスケッチの移植。元はESP32-S3+I2S(MAX98357A)向けで、`-tags cores3`ならI2Sで鳴る)、jukebox(著作権の切れた3曲をメロディ+伴奏の2声部、音符ごとのADSRで演奏。ボタンで次の曲へ) |

`examples/`は本家と同じフォルダ構成で、本家の`examples/01.Basics/Sinewave/Sinewave.ino`の移植は`examples/01.Basics/Sinewave/main.go`にあります。どのサンプルもボードに依存しないコードで、出力先は[`board`](../board)パッケージがビルドタグで選びます(`-tags cores3`でM5Stack CoreS3の内蔵スピーカーなど)。`_HIFI`の2本は、本家ではAVRの2ピンPWMモードを選ぶためのものですが、TinyGo版の出力は8ビット固定なので通常版と同じ音になります。

本家の残り(`03.Sensors`以降の90本ほど)は未移植です。移植したい場合の手順は[05-extending.md](05-extending.md)を参照してください。

## まとめ: 移植方針の考え方

このリストを見て「思ったより移植していないものが多い」と感じるかもしれませんが、これは意図的な判断です。本家Mozziは非常に大規模なライブラリで、全ファイルを1セッションで機械的に移植することは現実的ではありませんでした。代わりに:

- **コアアーキテクチャ(スケジューラ、固定小数点演算、基本的なオシレータ/エンベロープ/フィルタ/ディレイ)は忠実に移植**し、実際に音が出る土台を作る
- **波形テーブルは機械的に自動変換できるため、コストが低い割にリターンが大きく、全部移植**
- **ニッチな機能・センサー入力・サンプル再生・特定エフェクト**は、需要が出たときに同じパターンで追加できるよう、あえて後回しにした

このリストにある「❌ 未移植」の項目は、今後の機能追加の候補リストとしても使えます。
