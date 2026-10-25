# mozzi(TinyGo版)

English: [README.en.md](README.en.md)

Arduino 用の音声合成ライブラリ [Mozzi](https://github.com/sensorium/Mozzi)([ドキュメント](https://sensorium.github.io/Mozzi/))の、[TinyGo](https://tinygo.org/) への移植です。Mozzi の設計と主要な部品を移植し、**Raspberry Pi Pico(RP2040)** と **ESP32-S3** 向けのハードウェア出力(バックエンド)を備えています。

## 移植の範囲

中核部分の移植であり、全ファイルの逐語訳ではありません。

本家 Mozzi は大きなライブラリです。コアのヘッダが約 60、波形テーブルが約 60、ハードウェアバックエンドが約 15 種類(AVR/ESP32/SAMD/RP2040/STM32/Teensy など)、サンプルスケッチが 100 以上あります。これを全部機械的に移植するのは現実的ではないので、この移植では次のようにしています。

- **中核の設計**(制御レート/オーディオレートのスケジューラ、固定小数点演算、主なオシレータ・エンベロープ・フィルタ・ディレイのクラス)を、Go らしい書き方で忠実に移植しています。
- `Mozzi/tables/*.h` の **「フラットな」波形テーブル 59 個をすべて**機械的に変換しています([波形テーブル](#波形テーブル)を参照)。本家にあるテーブルのうち `BandLimited_*` ディレクトリ以外のものは、すべて使えます。
- **ハードウェアバックエンドを 4 つ**用意しています(RP2040 の PWM 出力、RP2040 + 外付け I2S DAC/アンプ、ESP32-S3 の PWM 出力、ESP32-S3 + I2S DAC/アンプ(M5Stack CoreS3 の内蔵スピーカーを含む))。どれを使うかは小さな [`board`](board) パッケージがビルド時に選ぶので、どのスケッチもどのボード向けにもビルドできます。
- **サンプルスケッチを 24 個、本家の `examples/` と同じフォルダ構成で**用意しています。本家の `01.Basics` と `02.Control` の全部、FMsynth、ADSR_Audio_Rate_Envelope、AudioDelay、ResonantFilter に加え、`mozzi/song` を使った独自の音楽スケッチが 4 つあります(`mozzi/song` は本家 Mozzi にはありません。[メロディを鳴らす](#メロディを鳴らす)を参照)。[サンプル](#サンプル)を参照してください。
- それ以外のもの(センサー入力の補助クラスの大半、ほかのハードウェアバックエンド、サンプル再生の大半、リバーブ、MIDI 入出力、新しい UFix/SFix 型の `FixMath` ライブラリなど)は、意図的に**移植していません**。[移植していないもの](#移植していないもの)を参照してください。

目指しているのは、今日からシンセを作れるしっかりした土台と、拡張するときに真似できる一貫したパターンです。既存の Mozzi スケッチすべての置き換えではありません。

## 必要なもの

- [TinyGo](https://tinygo.org/) 0.41 以降(0.41.1 で開発・テスト)。`backend/esp32s3i2s`(つまり M5Stack CoreS3)には 0.42 以降が必要です。0.42 で追加された ESP32-S3 の GDMA レジスタ定義を使うためです(0.42.0 でテスト)。
- 下のいずれかのボード

## 対応ボード

| ボード / チップ | バックエンド | 状況 |
|---|---|---|
| **Raspberry Pi Pico** などの RP2040 ボード。ピン 1 本から PWM 出力 + RC フィルタ | [`backend/rp2040pwm`](backend/rp2040pwm) | 対応 |
| **任意の RP2040 ボード + 外付け I2S DAC/アンプ**(例: **Waveshare RP2040-Zero + MAX98357A**) | [`backend/rp2040i2s`](backend/rp2040i2s) | 対応 |
| **ESP32-S3**(Xtensa LX7)。ピン 1 本から PWM 出力 + RC フィルタ | [`backend/esp32s3pwm`](backend/esp32s3pwm) | 対応 |
| **ESP32-S3 + I2S DAC/アンプ**(例: MAX98357A、**M5Stack CoreS3** の内蔵 AW88298 スピーカーアンプ、**TinyGo Conf 2026 バッジ**) | [`backend/esp32s3i2s`](backend/esp32s3i2s) | 対応(GDMA のレジスタ定義のため TinyGo 0.42 以降が必要)。ESP32-S3 の I2S は DMA 経由でしかデータを受け取れないので、このバックエンドは GDMA バッファを輪にして流し続け、「バッファ送信完了」の GDMA 割り込みで各バッファを補充します。[`docs/04-hardware-backends.md`](docs/04-hardware-backends.md) を参照 |
| ESP32-C3(RISC-V)。ピン 1 本から PWM 出力 | -- | 未提供。ただし `esp32s3pwm` からの移植は簡単なはずです。TinyGo 0.41 には C3 向けにも同じ LEDC PWM ドライバと、動作する `interrupt.New` があります。ここではまだ誰も必要としていません。(C3 の I2S も S3 と同じ DMA 専用の設計なので、I2S バックエンドは `esp32s3i2s` にならえば作れます) |
| 初代 ESP32(Xtensa LX6。無印の「ESP32 DevKitC」など) | -- | **現状では不可能**: TinyGo 0.41 の `machine` パッケージにはこのチップ用の PWM(も DAC も)ドライバがなく、`runtime/interrupt` にもこのチップ向けの `interrupt.New` がありません。どちらも Xtensa/`device/esp` のレジスタを直接叩いてゼロから書く必要があり、誰も検証していません |
| Mozzi/TinyGo が対応するそのほかのチップ(AVR、SAMD、STM32、Teensy、esp8266 など) | -- | 未移植。拡張ポイントは `engine.Backend` です([拡張する](#拡張する)を参照) |

2 つの PWM バックエンドは同じ手法を使っています。PWM のデューティ比を高速な 1 ビット DAC として使い(本家 Mozzi の AVR 向け `MOZZI_OUTPUT_PWM` モードと同じ)、この移植で手作りした周期的なハードウェアタイマー割り込みで駆動します(RP2040 にも ESP32-S3 にも、TinyGo には「オーディオ用の繰り返しタイマー」の API がそのままの形ではないため)。

`rp2040i2s` は、RP2040 の PIO ブロックで本物の I2S DAC/アンプを駆動します(I2S-over-PIO をゼロから書き直すのではなく、コミュニティの [`github.com/tinygo-org/pio`](https://github.com/tinygo-org/pio) ライブラリの `piolib.I2S` を使っています)。I2S のビットクロック自体がハードウェアでサンプル出力のペースを決めるので、このバックエンドはタイマー割り込みの代わりに、PIO の FIFO を補充し続けるだけの goroutine を動かします。

`esp32s3i2s` はこの方法が使えません(ESP32-S3 の I2S は GDMA 経由でしかデータを受け取らない)。そのため DMA バッファの輪を流し続け、1 つ送り終わるたびに上がる GDMA 割り込みでそのバッファを補充します。

いずれの場合も `engine.Backend` はメソッド 1 つのインターフェースです。PWM/DAC/I2S の出力と、サンプルを渡すペースを決める*何らかの*手段(割り込みでもそれ以外でも)があるボードなら、移植先として十分ありえます。[拡張する](#拡張する)を参照してください。

## 配線

**PWM バックエンド**(`rp2040pwm`、`esp32s3pwm`): 音は GPIO ピン 1 本から PWM として出ます。ピンとアンプ/スピーカー/オシロスコープの間に簡単な RC ローパスフィルタを入れて、PWM の搬送波(約 250kHz で、可聴域よりずっと高い)をならし、アナログ信号にします。

```
audioPin ---[ 1k ]---+--- アンプ / スピーカー / オシロスコープへ
                      |
                    [10nF]
                      |
                     GND
```

[`board`](board) を通すと、PWM は RP2040 ボードでは GPIO15、ESP32-S3(`-tags esp32s3pwm`)では GPIO4 から出ます。ほかのピンを使うには、`board/board_rp2040pwm.go` / `board/board_esp32s3pwm.go` の `audioPin` を変えてください。

インピーダンスの低いスピーカーを(ライン入力ではなく)GPIO ピンから直接鳴らすと、上の RC フィルタを通しても、ピンが安全に流せる電流をはるかに超えてしまいます(8Ω のスピーカーに 3.3V なら約 412mA!)。その場合は、RC フィルタの代わりに NPN トランジスタ 1 個をローサイドスイッチとして入れてください。RC フィルタは不要です(スピーカー自体の慣性とトランジスタのスイッチング速度で、PWM の搬送波は自然に減衰します)。

```
        3V3
         |
      [ スピーカー ]
         |
       コレクタ
   NPN (2N3904 / 2SC1815 など小信号用なら何でも)
       ベース ----[ 1kΩ ]---- audioPin
       エミッタ
         |
        GND
```

(必要に応じて、スピーカーと並列にダイオードを入れると(カソードを 3V3、アノードをコレクタ側に)、コイルの逆起電力を吸収できます。安上がりな保険ですが、ちょっと試すだけなら不要です)

**`rp2040i2s`**(例: Waveshare RP2040-Zero + MAX98357A I2S アンプ): RC フィルタは不要ですが、ピン 1 本の代わりに GPIO 3 本と共通の GND が必要です。

```
RP2040 ボード        MAX98357A
------------        ---------
3V3            -->  VIN
GND            -->  GND
GPIO2          -->  DIN   (音声データ)
GPIO3 (BCLK)   -->  BCLK
GPIO4 (BCLK+1) -->  LRC
(SD は未接続のままにするか、約 1MΩ で VDD に吊ると、L+R の平均のモノラル出力になる)
                     スピーカー端子 --> スピーカー
```

BCLK と LRC は*連続した* GPIO 番号(BCLK、BCLK+1 の順)にする必要があります。これは `rp2040i2s` が使う PIO プログラムの制約で、MAX98357A の制約ではありません。ビルドは `-tags rp2040i2s` で行います([サンプル](#サンプル)を参照)。

**M5Stack CoreS3**: 配線は不要です。[M5Stack CoreS3](#m5stack-cores3) を参照してください。

## サンプル

`examples/` は本家 Mozzi の `examples/` フォルダと同じ構成です。たとえば `examples/01.Basics/Sinewave` は `Mozzi/examples/01.Basics/Sinewave/Sinewave.ino` の移植です。スケッチにはハードウェアを書き込まず、音声出力を `board.NewOutput` から受け取ります。[`board`](board) が、ビルド対象のボードに合った出力を用意します。

| ボード | ビルドフラグ |
|---|---|
| M5Stack CoreS3 / CoreS3 SE の内蔵スピーカー | `-target=esp32s3-generic -tags cores3` |
| ESP32-S3、GPIO4 から PWM 出力 + RC フィルタ | `-target=esp32s3-generic -tags esp32s3pwm` |
| Raspberry Pi Pico / 任意の RP2040、GPIO15 から PWM 出力 + RC フィルタ | `-target=pico`(またはそのボードの RP2040 ターゲット) |
| RP2040 + I2S DAC/アンプ。例: RP2040-Zero + MAX98357A(DIN=GPIO2、BCLK=GPIO3、LRC=GPIO4) | `-target=waveshare-rp2040-zero -tags rp2040i2s` |
| TinyGo Conf 2026 バッジ(ESP32-S3 DevKit + MAX98357A)のスピーカー | `-target=esp32s3-generic -tags tinygoconf2026badge` |

たとえば `mozzi/` ディレクトリで次のようにします。

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave   # M5Stack CoreS3
tinygo flash -target=pico ./examples/01.Basics/Sinewave                                                # Raspberry Pi Pico
go mod tidy   # 最初の -tags rp2040i2s のビルドの前に 1 回だけ: github.com/tinygo-org/pio を取得する
tinygo flash -target=waveshare-rp2040-zero -tags rp2040i2s ./examples/01.Basics/Sinewave               # RP2040-Zero + MAX98357A
```

Sinewave は 440Hz の音を鳴らし続けるだけです。ツールチェーン、ボード、配線がすべて動くかを最初に確かめるのにちょうどよいサンプルです。ESP32-S3 向けのビルドでボードのタグ(`cores3`、`tinygoconf2026badge`、`esp32s3pwm` のいずれか)を付け忘れると、どれかを推測するのではなく、直し方を示すコンパイルエラーで止まります。ボードごとに使うピンが違い、PWM 構成の GPIO4 は CoreS3 では microSD カードのチップセレクト線だからです。

| サンプル | |
|---|---|
| **01.Basics** | |
| `Sinewave` | 440Hz のサイン波を鳴らし続ける |
| `Sinewave_HIFI` | 同上。本家の HIFI 出力モードに相当するものはここにはない(ファイルを参照) |
| `Control_Gain` | 制御レートで音量が段階的に下がっていくサイン波 |
| `Skeleton` | 自分のスケッチを書き始めるための、音の出ない最小構成 |
| `Skeleton_Multi` | 2 つのファイルに分けたスケッチ |
| `Table_Resolution` | 6 種類の大きさのサインテーブルを順に鳴らし、エイリアシングを聞き比べる |
| `Vibrato` | 位相変調によるビブラート |
| `Vibrato_Midi_Note` | 制御レートで MIDI ノートを変調するビブラート |
| **02.Control** | |
| `Control_Echo_Theremin` | 光センサーで演奏するエコー付きテルミン。`board.A0` に光センサー(CdS)が必要 |
| `Control_Oscil_Wash` | 音量をゆっくり変化させた 8 つの音が重なるアンビエント |
| `Control_Tremelo` | トレモロ。ゲインを Line でオーディオレートでなめらかにしている |
| `EventDelay` | 1 秒ごとにオン/オフする音 |
| `Line_Gliss` | のこぎり波のグリッサンド。位相の増分を Line で補間する |
| `Line_Gliss_Double_32k_HIFI` | 少しずらした 2 つのグリッサンドを、オーディオレート 32768Hz で鳴らす |
| `Metronome_SampleHuffman` | ハフマン圧縮した親指ピアノの音を、メトロノームに合わせてランダムに鳴らす |
| `Stop_Start` | `board.Button` のボタンでエンジンを止めたり再開したりする |
| **06.Synthesis** `FMsynth` | 音程がふらつく 2 オペレーターの FM シンセ |
| **07.Envelopes** `ADSR_Audio_Rate_Envelope` | ランダムな ADSR エンベロープ付きの音 |
| **09.Delays** `AudioDelay` | 変調付きディレイライン(フランジャー風) |
| **10.Audio_Filters** `ResonantFilter` | ノイズっぽい波形テーブルにかけたレゾナンス付きローパスフィルタ |
| **TinyGo_Extras**(本家にはないもの) | |
| `fanfare` | 録音から書き起こした JRA の競馬ファンファーレ(`mozzi/song`) |
| `furuhata_op` | テレビ番組のテーマの冒頭のオスティナート(一部のみ。ファイルのドキュメントコメントを参照) |
| `fanfare_synth` | 同じファンファーレを、少しずらした 3 つののこぎり波と ADSR で鳴らす |
| `jukebox` | パブリックドメインの曲(歓喜の歌、きらきら星、エリーゼのために)を 2 声部で、各音に ADSR を付けて鳴らす。`board.Button` で次の曲へ |

入力が必要な 2 つのスケッチは、`board` がボードごとに定めたピンを使います。`board.A0`(本家がアナログ入力 0 を読むところ)と `board.Button`(押しボタン。`board.ConfigureButton`/`board.ButtonPressed` で読む)です。どの GPIO かは [`board`](board) パッケージのドキュメントを参照してください。TinyGo Conf 2026 バッジでは、ジョイスティックの X 軸と SW1 にすでに割り当ててあります。

## TinyGo Conf 2026 バッジ

[TinyGo Conf 2026 バッジ](https://github.com/tinygo-keeb/tinygo-conf-2026-badge)は、MAX98357A I2S アンプとスピーカー(BCLK=GPIO45、LRC=GPIO21、DIN=GPIO47)、ジョイスティック、スイッチ 2 つを載せた基板に ESP32-S3 DevKit を組み合わせたものです。どのサンプルも `-tags tinygoconf2026badge` でビルドできます。

```sh
tinygo flash -target=esp32s3-generic -tags tinygoconf2026badge -port /dev/ttyACM0 ./examples/01.Basics/Sinewave
```

- DevKit の USB ポートのうち、ESP32-S3 に直結している方(Espressif の「USB JTAG/serial debug unit」として見える)を PC につないでください。TinyGo はこちらから書き込み、`println` もこちらに出ます。USB-UART チップ(例: CH343、1a86:55d3)経由のもう一方のポートには、`println` の出力は何も出ません。
- 既定の音量は 255 中 32 で、バッジ自身のファームウェアと同じです。バッジの MAX98357A はゲイン 12dB で動いていて、あまり強く鳴らすと音が割れます。`-ldflags="-X mozzi/board.volume=64"` か、コードから `board.SetVolume` で変えられます。
- `board.A0` はジョイスティックの X 軸です(GPIO7。中央で 1023 中約 512、動く範囲は左右それぞれ全体の 1/4 程度に制限されている)。そのため [`Control_Echo_Theremin`](examples/02.Control/Control_Echo_Theremin) はスティックを動かして演奏します。`board.Button` は SW1(GPIO13、押すとピンが Low になる)なので、[`Stop_Start`](examples/02.Control/Stop_Start) はボタンを押すと 1 秒間音が止まります。
- バッジのファームウェアは、ディスプレイなどのドライバのために `-target=esp32s3-box-3` を勧めています。mozzi はそれらを使わないので、`esp32s3-generic` で動きます。

## M5Stack CoreS3

CoreS3(と CoreS3 SE)は、AW88298 I2S アンプの先にスピーカーを内蔵した ESP32-S3 ボードなので、配線はまったく不要です。どのサンプルも `-tags cores3` でビルドすれば、内蔵スピーカーから鳴ります。[`board/cores3`](board/cores3) が、ボード内部の I2C バス経由でアンプを起動し(AW9523B IO エキスパンダでアンプのリセット線を解除し、M5Unified にならって AW88298 を設定する)、スピーカーの I2S ピン(BCK=G34 / WS=G33 / DOUT=G13)で [`backend/esp32s3i2s`](backend/esp32s3i2s) を使って駆動します。

**書き込み。** CoreS3 を USB-C でつなぎ(Espressif の「USB JTAG/serial debug unit」として見え、ドライバは不要)、`mozzi/` ディレクトリで次のようにします。

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/02.Control/Control_Oscil_Wash
```

- CoreS3 専用の TinyGo ターゲットはありません。`esp32s3-generic` と `-tags cores3` の組み合わせを使います。CoreS3 向けに `-tags esp32s3pwm` でビルドしないでください。PWM を GPIO4 から出しますが、そこは CoreS3 では microSD カードのチップセレクト線です。
- `-port` はボードのシリアルポートです。Linux では `/dev/ttyACM0`(や `ACM1` など)、macOS では `/dev/cu.usbmodem*`、Windows では `COMn` です。Linux では、ポートを所有するグループ(`dialout`、Arch 系のディストリビューションでは `uucp`)にユーザーが入っている必要があります。`ls -l /dev/ttyACM0` で確認してください。
- 書き込むと、最初にボードのフラッシュが消去されます。M5Stack の工場出荷時ファームウェア(や UIFlow など)は、最初の書き込みで消えます。M5Burner で元に戻せます。
- スケッチは書き込み直後と、電源を入れるたびに始まります。

**音量。** 出力レベルは 0(無音)から 255(フルスケール、既定値)までで、振幅に比例します。128 で約半分(-6dB)、64 で 1/4(-12dB)です。どのサンプルも、ソースを編集せずにビルド時に指定できます。

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -ldflags="-X mozzi/board.volume=128" -port /dev/ttyACM0 ./examples/01.Basics/Sinewave
```

または、コードからいつでも(たとえばフェードのために `UpdateControl` から)`board.SetVolume(128)` で変えられます。音量は、バックエンドが 8 ビットのサンプルを 16 ビットの I2S ワードに広げるときに掛けるので、下げても分解能は落ちません(`UpdateAudio` の中でサンプルを小さくすると落ちます)。ほかのボードでは効果はありません。

その下で、`board/cores3` はアンプを M5Unified の設定より 18dB 低く動かしています(I2S 入力で -6dB、音量段で -12dB)。M5Unified はアンプをフルゲインで動かしますが、フルスケールの信号は送りません。一方 mozzi のスケッチはフルスケールをよく出すので、フルゲインではアンプがクリップしていました。出力段が止まっては復帰するのを繰り返し、音程のある音ではなく、ポップコーンが弾けるようなパチパチという音に聞こえていました。18dB 下げると、音量 255 でフルスケールの音を鳴らしてもきれいに鳴ることを測定で確かめたので、255 がクリップしない最大の音量です。

**音質。** 内蔵スピーカーは小さく、低い純音(`Sinewave` の 440Hz)は少しこもって聞こえ、高い音ほどクリアに聞こえます。これはスピーカーによるものです。M5Stack 自身の M5Unified の `M5.Speaker.tone()` でも 440〜3520Hz で同じように聞こえ、本家 Mozzi でも同じです。8 ビットのサンプル、16384Hz のレート、I2S クロックのジッタが原因でもありません(16 ビットのサンプル、16kHz、ジッタのない 15625Hz のどれも同じように聞こえました)。

**入力。** `board.A0` は GPIO8、`board.Button` は GPIO9 で、黒い Grove ポート B の 2 本の信号ピンです。ポート B の電源ピンは **5V** で、ESP32-S3 のピンには耐えられません。センサーやボタンは、代わりに M-Bus ヘッダの 3.3V につないでください。

アンプの 1.8V 電源(AXP2101 PMIC の ALDO1)は既定でオンになっているので、`board/cores3` は読むだけです。それについての警告が USB シリアルに出た場合、スピーカーは鳴りません。ここにあるものは、CoreS3 のディスプレイは一切駆動しません。

**何も聞こえない、またはパチパチいう場合。** まず書き込み自体が成功したかを確かめてください。コマンドには `-tags cores3` が必要で、成功すると最後に `MD5 verified` / `Device reset.` と出ます。ビルドエラー(たとえば `boardNotSelected_buildWith_tags_cores3_tinygoconf2026badge_or_esp32s3pwm...` という名前が出るもの)は何も書き込まれていないことを意味し、ボードは以前のプログラムのまま動いています。次に [`tools/cores3-check`](tools/cores3-check) を書き込んでください。440Hz の音を鳴らしながら、音声経路の各段階の状態を 1 秒ごとに USB シリアルに報告します。

```sh
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./tools/cores3-check
tinygo monitor -port /dev/ttyACM0
```

```
---- cores3-check: playing a 440Hz test tone ----
audio out  : 16374 samples/s (expect ~ 16384 ) OK
tone level : 127 of 127 at volume 255 of 255 OK
I2S data   : 141947 edges/s on DOUT (G13) OK
amp supply : AXP2101 ALDO1 on OK
amp        : enabled yes | PLL locked yes | clock stable yes | no clock no OK
amp output : clipping 0 of 11 reads | output stage stopped 0 of 11 OK
```

最後の行は、1 秒に約 10 回読む中で、アンプがクリップや出力段の停止を報告した回数です。パチパチという音はここに現れます。全部の行が OK ならボードは鳴っていて、スケッチが無音なのはスケッチ自体の動きによるものです(意図的に静かだったり無音の時間があるものもあります。`EventDelay` は 1 秒おきに無音になり、`Skeleton` は常に無音です)。自分のスケッチでも、`cores3.ReadSpeakerStatus()` で同じアンプの状態を表示できます。

**本家 Mozzi との比較。** 本家のサンプルも同じ CoreS3 で無改造のまま動かして、聞き比べられます。`examples/` は本家と同じ構成なので、対応するサンプルは同じパスにあります。[`tools/cores3-original/build.sh`](tools/cores3-original/build.sh) は、`../Mozzi/examples` からサンプルをコピーし、[`mozzi_cores3.h`](tools/cores3-original/mozzi_cores3.h) を追加して(スピーカーのピンでの I2S 出力、この移植と同じ 16384Hz のオーディオレート(本家の ESP32 の既定は 32768Hz)、`board/cores3` と同じアンプ設定)、arduino-cli でビルドして書き込みます。`esp32:esp32` コア 3.x を入れた arduino-cli と、Mozzi 2.x が依存する FixMath ライブラリが必要です。

```sh
# mozzi/ で実行。ARDUINO_CLI の既定は PATH 上の arduino-cli
ARDUINO_CLI=/path/to/arduino-cli tools/cores3-original/build.sh 01.Basics/Sinewave /dev/ttyACM0    # 本家
tinygo flash -target=esp32s3-generic -tags cores3 -port /dev/ttyACM0 ./examples/01.Basics/Sinewave   # この移植
```

(FixMath は `arduino-cli lib install FixMath` で入れるか、`LIBRARIES` を https://github.com/tomcombriat/FixMath のチェックアウトに向けてください)ポートの引数を省くと、書き込まずにビルドだけします。本家の `01.Basics` と `02.Control` のうち、2 つの `_HIFI` スケッチ以外はすべてこの方法でビルドできます(この 2 つは AVR の 2 ピン PWM 出力モードを強制しますが、本家は ESP32 ではそれに対応していません)。ビルドできるもののうち 2 つは、本家が Arduino のピン番号で指定しているハードウェアを読みますが、CoreS3 ではそれが別のものを指します。`Control_Echo_Theremin` はアナログピン 0(GPIO0 で、ESP32-S3 では ADC のピンではない)を、`Stop_Start` はピン 4(microSD のチップセレクト)を読みます。しかも本家の ESP32 版の `stopMozzi()` は実装されていません。移植版では代わりに `board.A0`/`board.Button` を使います。

出力段で分かっている違い: モノラルのスケッチでは、本家の ESP32 の I2S 出力はサンプルを左チャンネルに送って右には 0 を送りますが、`esp32s3i2s` は両方に送ります。アンプが 2 つのチャンネルの平均を鳴らす場合、本家のほうが約 6dB 小さく聞こえます。片方のチャンネルだけを鳴らす場合は同じになります。ランダムな要素のあるスケッチのうち、Mozzi の既定の乱数シードのままのもの(`ResonantFilter` など)は、両方で同じ列を生成します。`ADSR_Audio_Rate_Envelope` は、本家では起動時にハードウェアの乱数発生器でシードし直します(`randSeed()`)が、移植版は固定のシードを使うので、音の並びが違います。

## 構成

Mozzi の中核の設計は、この移植では次のように対応しています。

| Mozzi(C++)                              | この移植(Go)                          |
|-------------------------------------------|------------------------------------------|
| `setup()` / `loop()` + `audioHook()`      | `main()` + `engine.Start` + `engine.AudioHook()` のループ |
| `updateControl()` / `updateAudio()`       | 自分の型の `UpdateControl()`/`UpdateAudio()` メソッド(`engine.Sketch` を実装する) |
| `MozziGuts.hpp`(スケジューラ、出力バッファ) | [`engine`](engine) |
| プラットフォームごとの `MozziGuts_impl_*.hpp` | [`backend/rp2040pwm`](backend/rp2040pwm)、[`backend/rp2040i2s`](backend/rp2040i2s)、[`backend/esp32s3pwm`](backend/esp32s3pwm)、[`backend/esp32s3i2s`](backend/esp32s3i2s)(すべて `engine.Backend` を実装) |
| `AudioOutput.h`(`MonoOutput`/`StereoOutput`) | [`audioout`](audioout) |
| `mozzi_fixmath.h`                          | [`fixmath`](fixmath) |
| `mozzi_rand.h`                             | [`rand`](rand) |
| `mozzi_midi.h`                             | [`midi`](midi) |
| `Oscil.h`、`Phasor.h`                      | [`oscil`](oscil) |
| `Line.h`                                   | [`line`](line)(ジェネリックな `Line[T]` 1 つ。下を参照) |
| `ADSR.h`、`Ead.h`、`Smooth.h`              | [`envelope`](envelope) |
| `EventDelay.h`                             | [`eventdelay`](eventdelay) |
| `Metronome.h`                              | [`metronome`](metronome) |
| `RollingAverage.h`                         | [`rollingaverage`](rollingaverage) |
| `mozzi_analog.h`(`mozziAnalogRead`)       | [`analog`](analog)(本家が ESP32/RP2040 でそうしているのと同じく、待ちのある読み取り) |
| `SampleHuffman.h`、`samples/thumbpiano_huffman/*.h` | [`samplehuffman`](samplehuffman)、[`samples/thumbpianohuffman`](samples/thumbpianohuffman)(生成したもの。[拡張する](#拡張する)を参照) |
| `ResonantFilter.h`                         | [`filter`](filter) |
| `AudioDelay.h`、`AudioDelayFeedback.h`、`ControlDelay.h` | [`delay`](delay)(`ControlDelay` は制御レートで使う `AudioDelay[T]` にすぎない) |
| `tables/*.h`                               | [`tables`](tables)(生成したもの。下を参照) |
| *(本家に相当するものはない)*                  | [`song`](song)。小さな単旋律のメロディシーケンサ。[メロディを鳴らす](#メロディを鳴らす)を参照 |
| ボードの選択(`MOZZI_AUDIO_MODE` など)    | [`board`](board)。ビルド時にバックエンドと入力ピンを選ぶ([サンプル](#サンプル)を参照)。[`board/cores3`](board/cores3) は M5Stack CoreS3 のスピーカーを準備する |

典型的な Mozzi のスケッチ:

```cpp
Oscil <SIN2048_NUM_CELLS, MOZZI_AUDIO_RATE> aSin(SIN2048_DATA);

void setup(){ startMozzi(); aSin.setFreq(440); }
void updateControl(){}
AudioOutput updateAudio(){ return MonoOutput::from8Bit(aSin.next()); }
void loop(){ audioHook(); }
```

は、次のようになります。

```go
type sketch struct{ aSin *oscil.Oscil }
func (s *sketch) UpdateControl() {}
func (s *sketch) UpdateAudio() audioout.Mono { return audioout.From8Bit(int16(s.aSin.Next())) }

func main() {
    s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
    s.aSin.SetFreq(440)
    out, _ := board.NewOutput(audioRate)
    engine.Start(s, audioRate, controlRate, out)
    for { engine.AudioHook() }
}
```

実際に動く版は [`examples/01.Basics/Sinewave`](examples/01.Basics/Sinewave) を参照してください。

### テンプレートがフィールド(とジェネリクス)になった理由

本家は C++ のテンプレート(`Oscil<NUM_CELLS, RATE>`、`Line<T>`、`ADSR<RATE1, RATE2, T>` など)を多用して、シフトや割り算をコンパイル時に定数畳み込みしています。数 MHz の 8 ビット AVR では、これが大きく効きます。RP2040(133MHz 以上のデュアル Cortex-M0+)では、その余裕はもう制約になりません。そのため、テンプレート引数の大半は普通のコンストラクタ引数/フィールドにしました(たとえば [`oscil.New`](oscil/oscil.go))。Go のジェネリクスがテンプレートにすっきり対応し、実行時のコストもほとんどないところ(`Line[T]`、`AudioDelay[T]`)では、ジェネリクスを使っています。

## 波形テーブル

`Mozzi/tables/*.h` にある 59 個のテーブル(`BandLimited_SAW/SQUARE/TRI` サブディレクトリは除く。下を参照)はすべて、[`tools/gentables`](tools/gentables) で機械的に、`//go:embed` したバイナリを持つ Go ソースに変換しました。これは本家の PROGMEM/`pgm_read_byte` の手順をただ移植したものではなく、*より良い*ものです。RP2040 は(TinyGo が対応するほかの Cortex-M ターゲットと同じく)フラッシュを直接アドレス指定できるので、埋め込んだバイトスライスはそのままフラッシュに置かれ、RAM を一切使いません。59 個のテーブルすべてを、実行時に変わる添字で参照する形(つまりコンパイラが定数畳み込みで消せない形)でビルドして確かめました。フラッシュはテーブルの実際の大きさだけ増え、RAM はまったく増えません。

テーブル名は元のヘッダの配列名に従っています(`SIN2048_DATA` -> `tables.Sin2048`、`WAVESHAPE_SIGMOID_DATA` -> `tables.WaveshapeSigmoid` など)。生成したものの全一覧は [`tables/doc.go`](tables/doc.go) を参照してください。

**既定では含めていないもの**: `BandLimited_SAW/SQUARE/TRI` のテーブル群です(エイリアシングを抑えた `MetaOscil` 風の再生のために、それぞれ複数の解像度がある)。合計約 4MB と、ほかのテーブル全部を合わせたものよりずっと大きく、しかもこの移植にはそれを使う `MetaOscil` も `AutoMap` も入っていません。必要なら自分で生成してください。

```sh
go run ./tools/gentables ../Mozzi/tables/BandLimited_SAW tables/bandlimited
```

(このジェネレータは `Mozzi/tables` のものに限らず、`CONSTTABLE_STORAGE(...)` のヘッダを集めた任意のディレクトリに使えます)

## メロディを鳴らす

[`song`](song) は本家 Mozzi に相当するもののない小さな追加で、「この曲を鳴らす」ようなスケッチ向けの単旋律のメロディシーケンサです。曲は `[]song.Event{Note, Beats}` のスライスで、`song.Note` の定数(`C4` から `B6` まで。五線譜での音名と同じ名前)と、休符の `song.Rest` で書きます。これを `song.Player` が `oscil.Oscil` で再生します。

```go
var melody = []song.Event{
    {song.G4, 1}, {song.C5, 1}, {song.E5, 1}, {song.G5, 2},
    {song.Rest, 0.5},
    {song.E5, 1}, {song.G5, 3},
}

// スケッチの中で:
player := song.NewPlayer(osc, controlRate, bpm, melody, true) // true: 繰り返す
func (s *sketch) UpdateControl() { s.player.Update() }
func (s *sketch) UpdateAudio() audioout.Mono {
    if !s.player.Playing() { return audioout.From8Bit(0) } // 休符のあいだは無音
    return audioout.From8Bit(int16(s.osc.Next()))
}
```

`Beats` は 4 分音符を 1 とした長さで、`bpm`(1 分あたりの 4 分音符の数で表したテンポ)とは独立しています。曲全体のテンポを変えるには定数を 1 つ変えるだけでよく、イベントを全部書き直す必要はありません。完全なスケッチは [`examples/TinyGo_Extras/fanfare`](examples/TinyGo_Extras/fanfare) を参照してください。その `melody` を作った手順も書いてあります。参考の録音から音声を取り出し(`ffmpeg -vn`)、音程とアタックを追跡して(numpy で書いた小さな自己相関による音程追跡と、エネルギーの包絡線によるアタック検出。scipy/librosa/aubio は不要)、推測ではなく測定した音と長さを得ています。[`examples/TinyGo_Extras/furuhata_op`](examples/TinyGo_Extras/furuhata_op) はその反対の例です。音声ではなく楽譜ソフトから書き出した PDF から書き起こしたメロディで、一番はっきりしていて何度も繰り返されるフレーズ以外は信頼できないほど読み取りが乱れていました。それらしい推測で穴を埋めることはせず、曲のどこまでを再現しているかをドキュメントコメントにはっきり書いてあります。

## 移植していないもの

全体を、手に負えるまとまった中核に保ち、手を広げすぎて中途半端な逐語訳にならないよう、範囲外にしたものです。どれも、すでにあるパターン(C++ のクラスを普通のフィールドで移植する、テーブルを生成する、新しい `engine.Backend` を書く)に沿って作れるので、拡張の出発点として妥当です。

- **ほかのハードウェアバックエンド**(AVR、SAMD、STM32、Teensy、esp8266、初代 ESP32 など)。何を提供しているか、何が実現できそうか、何が TinyGo 自体でまだ対応していないかは、上の[対応ボード](#対応ボード)の表を参照してください。拡張ポイントは `engine.Backend` です。
- **センサー/アナログ入力の補助クラスの大半**(`RCpoll.h`、`CapPoll.h`、`AutoRange.h`、`AutoMap.h`、`IntMap.h`、`RollingStat.h`、`Portamento.h`、`DCfilter.h`)、音声入力、`03.Sensors`/`04.Audio_Input` のサンプル群。`mozziAnalogRead`([`analog`](analog) として)と `RollingAverage.h` は移植しています。
- **サンプル再生の大半**(`Sample.h`、そのほかの `samples/*.h` の音声データ、`08.Samples` のサンプル)と、**`MetaOscil`/`WavePacket`/`OverSample`**。`SampleHuffman.h` と親指ピアノの音は移植しています。
- **`AudioDelayFeedback` の ALLPASS 補間モード**と、その int16 セル版のテンプレート。移植しているのは(既定で、よく使われる)int8 セルの LINEAR モードだけです。
- **`StateVariable.h`、`PDResonant.h`、`WaveFolder.h`、`WaveShaper.h`、`ReverbTank.h`** と、対応するサンプル群。
- **[サンプル](#サンプル)に挙げたもの以外のサンプル**。本家の `03.Sensors` から `13.External_Audio_Output` までの残り。
- **MIDI/シリアル通信**(`11.Communication` のサンプル)。
- **新しい `FixMath` ライブラリ**(本家の新しめのサンプルの一部が、従来の `mozzi_fixmath.h` の typedef の代わりに使っている、型安全な `UFix`/`SFix` の固定小数点テンプレート)。この移植は従来の API に従っており、移植したコードに必要なものはそれですべてまかなえます。`mozzi/fixmath` が従来の API に相当します。
- **ステレオ出力**(`audioout` に `AudioOutput`/`StereoOutput` はありますが、それを使うサンプルもバックエンドの配線もまだありません)。

## 忠実さについて

この移植の大部分は、本家のアルゴリズムを 1 行ずつそのまま訳したものです(同じシフト、同じ固定小数点の形式、同じ式)。いくつかの箇所は意図的に変えていますが、いずれも*より正しく*、より頑丈にする方向で、意味のあるところにはコメントで理由を書いています。

- `filter.Filter8`/`Filter16` は、一貫した 1 つの整数幅(状態は int32、積は int64)を使っています。本家の C++ が `IntegerType<>` で式の部分ごとに選んでいる幅は再現していません。それ自体にアルゴリズム上の意味はないためです。
- `midi.Q16n16MtoF` は、テーブルの末尾の 1 つ先を読む代わりに、参照する位置を範囲内に収めています(C++ のフラットなアドレス空間では無害ですが、Go ではスライスの範囲外アクセスで panic になります)。
- `audioout.Bits` は、論理的な出力の分解能を、本家のプラットフォームごとに設定できる `MOZZI_AUDIO_BITS` ではなく、すべてのバックエンドで 8 ビットに固定しています。理由はパッケージのドキュメントを参照してください。そのため、本家の `_HIFI` サンプル(AVR の約 14 ビットの 2 ピン PWM モードを使うのが目的)は、ここでは普通版と同じように鳴ります。
- `eventdelay.EventDelay` と `metronome.Metronome` は、待ち時間をミリ秒からオーディオのティックに変換するのを、設定したときではなく開始したときに行います。本家ではオーディオレートは定数ですが、ここでは `engine.Start` を呼ぶまで分からないからです。これで、`engine.Start` の前に作っておけます。

## 拡張する

- **新しいバックエンド**: `engine.Backend` の唯一のメソッド `Start(sampleRateHz uint32, pull func() audioout.Storage)` を実装します。ハードウェアが用意する周期的な割り込み、DMA 完了のコールバック(または `rp2040i2s` のように、ハードウェアへの書き込みで待たされる goroutine)から `pull()` を呼び、サンプルを DAC/PWM/I2S の出力に書きます。どの方式がハードウェアに合うかに応じて、`backend/rp2040pwm` と `backend/esp32s3pwm`(それぞれ約 150 行、タイマー割り込み駆動)、`backend/esp32s3i2s`(DMA の輪を、DMA 完了割り込みで補充)、`backend/rp2040i2s`(約 60 行、goroutine 駆動)がひな形になります。特に `esp32c3` は、`esp32s3pwm` を少し直すだけで済むはずです(LEDC PWM の API は同じ。そのパッケージのドキュメントにあるやり方で、空いている CPU 割り込み線と TIMG タイマーを見つける必要があります)。
- **新しいボード**: 専用のビルドタグを付けた `board/board_<名前>.go` を追加し、既存のものと同じように `NewOutput`、`A0`、`Button`、`applyVolume` を定義します(すでに既定のボードがあるターゲット向けなら、そのタグを `board_unselected.go` で除外します)。
- **新しい波形テーブル**: Mozzi 形式のヘッダ(または `CONSTTABLE_STORAGE(type) NAME[] = {...}` の配列を含む任意のファイル)をどこかに置き、`go run ./tools/gentables <ディレクトリ> <出力パッケージのディレクトリ>` を実行します。
- **新しいハフマン圧縮のサンプル音**: `go run ./tools/genhuffman <audio2huff.py のヘッダのあるディレクトリ> <出力パッケージのディレクトリ>` を実行します(`samples/thumbpianohuffman` はこれで作りました)。
- **新しいサンプルスケッチ**: `examples/01.Basics/Skeleton`(メロディを鳴らすものなら `examples/TinyGo_Extras/fanfare`)をコピーします。本家のサンプルの移植なら、本家と同じフォルダに置きます。

## 動作確認の状況

このリポジトリのものはすべて、`tinygo build`/`-size` で確認しています(ハードウェアに依存しないパッケージは、サンプルが使う以外のジェネリクスの具体化も含めて、`go build`/`go vet`/`go run` でも確認しています)。

`rp2040i2s` は実機(Waveshare RP2040-Zero + MAX98357A)で試しました。最初の版はまったく音が出ませんでした。配線の問題ではなく、本物のバグでした。出力のループは goroutine で動きますが、`for { engine.AudioHook() }`(どのサンプルのメインループもこれ)が一度も他に順番を譲らないため、TinyGo の協調スケジューラがその goroutine を一度も動かしていなかったのです。`engine.AudioHook()` が毎回 `runtime.Gosched()` を呼ぶようにして、一か所でまとめて直しました(そのドキュメントを参照)。このため、goroutine で動く `engine.Backend` は、スケッチのメインループに何も追加しなくても動きます。この修正のあと、同じボードで音が聞こえることを確認しました。

`esp32s3pwm` もその後、実機(M5Stack CoreS3 系のボード、TinyGo 0.42.0)で試しました。コンパイルは通るレジスタ直叩きのバックエンドにありがちな形で、うまく動きませんでした。サンプルの割り込みが 1 回だけ起きて、その後二度と起きなかったのです。TIMG の `ALARM_EN` ビットは、自動リロードを有効にしていてもアラームのたびに自動でクリアされるためでした。割り込みハンドラの中で設定し直すようにして直しました(`isr()` を参照)。そのあと、チップ上の自己テスト(スピーカーもオシロスコープも使わず、PWM ピンのパッドのレベルを GPIO の入力レジスタから読み戻す)で、16393Hz のサンプル割り込み(1MHz/61、公称 16384Hz)、それに追いつく `UpdateAudio`、約 64Hz の `UpdateControl`、約 440Hz のサイン波としてデューティ比が 0〜100% で振れるピンを測定しました。耳での確認はまだです。

`esp32s3i2s` は、同じ CoreS3 の AW88298 スピーカーアンプで確認しました([`examples/01.Basics/Sinewave`](examples/01.Basics/Sinewave)、`-tags cores3`)。音が聞こえることを確認済みです。GDMA 割り込みは毎秒 16373〜16384 サンプルを取り出し、BCLK/WS/データは約 32fs/fs で切り替わり、I2S を始めるとアンプ自身の状態レジスタが「クロックなし」から「PLL ロック、クロック安定」に変わり、440Hz の音がスピーカーから出ました。TinyGo Conf 2026 バッジ(MAX98357A)でも、サンプルを書き込んで音が鳴ることを確認しています。

`rp2040pwm` はまだ実機で試していません。ほかに 2 点、特に注意しておきます。

- `esp32s3pwm` は、Xtensa の割り込みマトリクスと TIMG タイマーのレジスタを直接操作しています(そのチップには土台にできる高水準の「繰り返しタイマー」の API がないため、TinyGo 自身のランタイムのソースを読んで、そこで使われているレジスタと割り込み線の決まりを割り出しました)。上の `ALARM_EN` のバグは、実際のシリコンでしか分からない種類のものです。
- `rp2040i2s` は、`github.com/tinygo-org/pio` の `piolib.I2S` という、この移植で書いたのではない、コミュニティが保守している外部の PIO プログラムを使っています。ゼロから書いたレジスタ直叩きのバックエンドよりは信頼してよいでしょうが、*このリポジトリでは*まだ検証していません。その周りのチャンネルの複製と 8 ビットから 16 ビットへの変換は、新しく書いたコードです。

最初の書き込みを本当のテストだと思ってください。

## ライセンス

本家 Mozzi と同じく、この移植は GNU Lesser General Public License(LGPL)v2.1 以降でライセンスされています。[`LICENSE.TXT`](LICENSE.TXT) / [`LICENCE.LGPL`](LICENCE.LGPL) を参照してください。
