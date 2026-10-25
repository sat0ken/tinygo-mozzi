# 対応マイコンとハードウェアバックエンドの内部実装

## 1. 対応状況まとめ

| ボード/チップ | バックエンド | 状態 | 実機確認 |
|---|---|---|---|
| Raspberry Pi Pico等RP2040ボード全般 | [`backend/rp2040pwm`](../backend/rp2040pwm) | 対応 | ❌ 未確認(ビルド確認のみ) |
| RP2040ボード + 外部I2S DAC/アンプ(例: Waveshare RP2040-Zero + MAX98357A) | [`backend/rp2040i2s`](../backend/rp2040i2s) | 対応 | ✅ **確認済み**(音出し成功) |
| ESP32-S3(Xtensa LX7)、PWM出力 | [`backend/esp32s3pwm`](../backend/esp32s3pwm) | 対応 | ⚠️ 実機(M5Stack CoreS3系)でチップ内自己診断により確認。サンプル割り込み16393Hz、PWMピンのデューティが0〜100%で約440Hzのサイン波として変化。初回は割り込みが1回しか発火しないバグ(TIMGの`ALARM_EN`が自動クリアされる)があり修正済み。耳での音出し確認はまだ |
| ESP32-S3 + I2S DAC/アンプ(MAX98357A、M5Stack CoreS3内蔵のAW88298等) | [`backend/esp32s3i2s`](../backend/esp32s3i2s) | 対応(TinyGo 0.42以降) | ✅ **確認済み**(M5Stack CoreS3の内蔵スピーカーで音出し成功。I2Sクロック/データの出力とアンプのPLLロックも確認。TinyGo Conf 2026バッジ(ESP32-S3 DevKit + MAX98357A、`-tags tinygoconf2026badge`)でもI2S信号の出力を確認)。詳細は[§6](#6-backendesp32s3i2sの内部実装) |
| ESP32-C3(RISC-V)、PWM出力 | — | 未実装 | TinyGo 0.41には同じLEDC PWMドライバと`interrupt.New`があるため、`esp32s3pwm`の軽い改造で対応できるはず(需要がなく未着手)。I2Sペリフェラルの設計はS3と同じくDMA専用なので、I2S対応は`esp32s3i2s`と同じ方式になる |
| 無印ESP32(Xtensa LX6、DevKitC等) | — | **対応不可** | TinyGo 0.41.1時点で`machine`パッケージにPWM/DACドライバが存在せず、`runtime/interrupt`にも`interrupt.New`のサポートがない。生のXtensaレジスタ操作から書く必要があり、検証手段もない |
| AVR, SAMD, STM32, Teensy, esp8266等 | — | 未移植 | `engine.Backend`が拡張ポイント |

**なぜRP2040とESP32-S3だけなのか**: どちらもTinyGoに(a) PWM出力用のドライバ、(b) カスタム割り込みを登録する`interrupt.New()`相当の仕組み、の両方が揃っていたためです。この2つが揃っていないと、正確なタイミングでの音声出力を自前で実装できません。

## 2. 共通の設計: `engine.Backend`インターフェース

```go
type Backend interface {
    Start(sampleRateHz uint32, pull func() audioout.Storage)
}
```

`Start()`が呼ばれたら、バックエンドは何らかの方法で「サンプルレートの周期で`pull()`を呼び、その結果をハードウェアに出力する」処理を開始する責任があります。**「何らかの方法」には2つのスタイルがあります**(詳細は[02-port-architecture.md](02-port-architecture.md#31-重要な落とし穴-runtimegosched))。

- **タイマー割り込み駆動**: `rp2040pwm`, `esp32s3pwm`
- **goroutine駆動**: `rp2040i2s`

## 3. `backend/rp2040pwm`の内部実装

PWMのデューティ比を使って、GPIOピン1本を簡易DACとして使う方式です(本家MozziのAVR `MOZZI_OUTPUT_PWM`モードと同じ考え方)。

### 3.1 PWM出力

TinyGoの`machine.PWM0`〜`machine.PWM7`(RP2040には8個のPWMスライスがある)を使い、`carrierHz`(既定250kHz、可聴域よりずっと高い)でPWMを回します。PWMの分解能(Top値)はTinyGoの`PWMConfig.Period`計算が自動的に決めます(低いキャリア周波数ほど分解能が上がる)。

### 3.2 サンプルクロック: TIMERのALARM1を使う理由

RP2040には`TIMER`ペリフェラルに4つの独立したアラーム(`ALARM0`〜`ALARM3`、それぞれ別のIRQ番号)があります。最初`ALARM0`(`IRQ_TIMER_IRQ_0`)を使おうとしたところ、ビルド時に以下のエラーが出ました。

```
handlers for interrupt 0 in multiple packages: mozzi/backend/rp2040pwm and machine
```

**TinyGoの`machine`パッケージ自身が、内部で(おそらく`time.Sleep`等のために)`ALARM0`/`IRQ_TIMER_IRQ_0`を既に使っていた**ためです。これを避けて`ALARM1`(`IRQ_TIMER_IRQ_1`)に切り替えることで解決しました([`backend/rp2040pwm/rp2040pwm.go`](../backend/rp2040pwm/rp2040pwm.go)のコメント参照)。

**教訓**: RP2040の4本のタイマーIRQのうち、どれかが`machine`パッケージに予約されている可能性があるので、新しいタイマー割り込みを使う前に一度ビルドして衝突がないか確認すること。

### 3.3 タイミングの仕組み

- `ALARM1`に「次にサンプルを出すべき時刻」を書き込んでおくと、`TIMERAWL`(1MHzでカウントアップするフリーランタイマー)がその値に達した時に割り込みが発生
- 割り込みハンドラ内で、次の時刻を(蓄積誤差が出ないよう固定小数点で)計算して再度`ALARM1`に書き込み、`pull()`で取得したサンプルをPWMのデューティ比に変換して書き込む
- もし処理が遅れて既に次の時刻を過ぎていたら、ループしてすぐにもう1サンプル処理する(本家のMOZZI_OUTPUT_EXTERNAL_TIMEDモードと同じ「追いつくまでループ」方式)

## 4. `backend/rp2040i2s`の内部実装

外部I2S DAC/アンプ(MAX98357A等)向け。RP2040のPIO(Programmable I/O)ブロックで本物のI2Sビットストリームを生成します。

### 4.1 ゼロから書かなかった理由

I2S-over-PIOのアセンブリプログラムを自分で書くのは(ビット単位のタイミング設計が必要で)リスクが高いため、コミュニティ製ライブラリ[`github.com/tinygo-org/pio`](https://github.com/tinygo-org/pio)の`piolib.I2S`を利用しています(`go.mod`に依存として追加済み)。信頼度は「本家Mozziの実装」ほどではありませんが、「ゼロから書いたレジスタ操作コード」よりは高いと判断しました。

### 4.2 なぜタイマー割り込みが不要なのか

PIOのステートマシンは、設定したクロック分周比に従って**ハードウェアが正確にビットクロックを生成**します。`piolib.I2S.WriteStereo()`はFIFOが埋まっていればブロック(内部で`gosched()`を呼びながら待つ)するため、「FIFOに書き込み続けるだけのgoroutine」を回すだけで、結果的に正確なサンプルレートでの出力が実現します。自前のタイマー割り込みは不要です。

### 4.3 モノラル→ステレオの扱い

MAX98357Aのようなモノラル出力I2Sアンプは、SDピンの状態でL/R/L+Rどれを再生するか選べます(未接続や特定の抵抗値でL+R平均=事実上のモノラル)。そのため、このバックエンドは同じサンプルをL/R両チャンネルに複製して送っています。

### 4.4 落とし穴: goroutineが実行されない問題

実機テストで最初「無音」になったバグの原因と修正は[02-port-architecture.md](02-port-architecture.md#31-重要な落とし穴-runtimegosched)で説明した通りです。**このバックエンド自体は正しく実装されていても、呼び出し側(`engine.AudioHook()`)がyieldしなければ動かない**という、TinyGoのgoroutineスケジューラ特有の罠でした。

### 4.5 疑われる問題: 重いシンセ(複数オシレータ)でテンポが遅くなる

単一オシレータのサンプル([`examples/01.Basics/Sinewave`](../examples/01.Basics/Sinewave)、`-tags rp2040i2s`。当時は専用のサンプルでした)では実機で正常動作を確認できましたが、3枚のオシレータ+ADSRを使う、より計算量の多いサンプル([`examples/TinyGo_Extras/fanfare_synth`](../examples/TinyGo_Extras/fanfare_synth)、`-tags rp2040i2s`)で「メロディのテンポが遅い」という報告がありました。

`engine`のメロディ/制御レートの進行は、「実時間」ではなく「`AudioHook()`が新しいサンプルを**生成**した回数」を数えて進む仕組みです。定常状態(出力バッファがほぼ満杯)であれば、これは実時間と一致するはずですが、**もしサンプル生成(=`UpdateAudio()`の計算+goroutine切り替えのオーバーヘッド)が実際の出力消費速度(既定16384Hz)に追いつけない場合、生成ペース自体が実時間より遅くなり、結果としてメロディ全体のテンポが遅く聞こえる**可能性があります。RP2040(Cortex-M0+, 125MHz, FPU無し)は、元のスケッチが動いていたESP32-S3(240MHz, デュアルコア, FPU有り)より非力なため、この仮説は妥当性があります。

**試した対策**: `run()`の内側ループを、1フレームずつではなく`batchFrames`(既定8)フレームまとめて`WriteStereo()`に渡すよう変更しました。`WriteStereo`/`TxPut`自体は(ハードウェアFIFOの空き待ちで)必要に応じてブロックするため実時間ペーシングは変わりませんが、goroutine切り替え・関数呼び出しのオーバーヘッドが1/16384秒ごとから1/(16384/8)秒ごとに減ります。

**注意**: これは実機での確証が取れていない、あくまで有力な仮説に基づく対策です。改善しない場合の次の一手としては、`batchFrames`をさらに増やす、または`audioRate`定数を(16384から8000などに)下げて必要なスループットそのものを減らす、といった調整が考えられます。`audioRate`を下げても、`oscil.New`/`envelope.NewADSR`/`engine.Start`が全て同じ定数を参照しているため音程はズレません(帯域が多少落ちるだけで、この用途では体感できない範囲のはずです)。

## 5. `backend/esp32s3pwm`の内部実装

ESP32-S3のLEDC(LED Controller)ペリフェラルをPWM出力に流用します。

### 5.1 PWM出力

TinyGoの`machine.PWM0`〜`PWM3`(ESP32-S3向けLEDCドライバ、TinyGo 0.41で追加された比較的新しい機能)をそのまま使えます。RP2040と違い、ESP32のLEDCは「どのピンにも自由にルーティングできる(GPIOマトリクス)」ため、ピンとPWMスライスの対応を気にする必要がありません。

### 5.2 サンプルクロック: TIMG0のTimer1を使う理由

ESP32-S3には`TIMG0`/`TIMG1`という2つのタイマーグループがあり、それぞれ`Timer0`(T0)と`Timer1`(T1)を持ちます。TinyGoのランタイム自身のソースコード([`runtime/runtime_esp32sx.go`](https://github.com/tinygo-org/tinygo/blob/main/src/runtime/runtime_esp32sx.go)、TinyGoのインストールディレクトリ内)を読んだところ、**`time.Now()`/`time.Sleep()`の実装のためにTIMG0の`Timer0`(T0)を既に使っている**ことがわかりました。

これと衝突しないよう、このバックエンドは**TIMG0の`Timer1`(T1)**を使っています。設定はT0と完全に対称(`SetT1CONFIG_*`, `T1LOADLO`, `T1ALARMLO`等)です。

### 5.3 割り込み番号(CPU interrupt line)の選択

ESP32-S3(Xtensa)は、RP2040とは違う2段階の割り込みモデルを持ちます。

1. 各ペリフェラル割り込み要因(~99種類)を、**割り込みマトリクス**(`INTERRUPT_CORE0`)経由で32本のCPU割り込みライン(0〜31)のどれかにマッピングする
2. TinyGoの`interrupt.New(cpuInt, handler)`は、このCPU割り込みライン番号を受け取る(ペリフェラル番号ではない)

これもランタイムのソースを調べたところ、CPU割り込みライン**8(USB)・9(ランタイムのタイマー)・10(GPIO)が既に使用中**でした。そのため、このバックエンドは**ライン12**を使っています(TinyGo 0.41時点で他に使われていないことを確認済み)。

```go
esp.INTERRUPT_CORE0.SetTG_T1_INT_MAP(audioTimerCPUInterrupt) // audioTimerCPUInterrupt = 12
interrupt.New(audioTimerCPUInterrupt, handleTimerInterrupt).Enable()
```

**教訓**: ESP32系で新しい割り込みを追加する場合、TinyGoのランタイム・machineパッケージが同じCPU割り込みラインやタイマーを既に使っていないか、ソースを確認すること。使用状況は`interrupt_esp32s3.go`(`runtime/interrupt`パッケージ内)にコメントで説明されています。

### 5.4 AUTORELOADモードで割り込みハンドラが単純になる理由

RP2040の`rp2040pwm`は「次の時刻を計算して都度アラームを再設定する」方式でしたが、ESP32のTIMGには**AUTORELOAD機能**があり、「指定した周期で自動的にカウンタをリセットしてアラームを再発火し続ける」設定が可能です。ただし、**AUTORELOADだけでは割り込みは1回しか発火しません**。`ALARM_EN`ビットはアラームが発生するたびにハードウェアが自動でクリアする(TRM上 R/W/SC)ため、`esp32s3pwm`の割り込みハンドラは割り込みフラグのクリアに加えて`ALARM_EN`を毎回セットし直しています(ESP-IDFのgptimerも同じことをしています)。当初はこの再アームが無く、実機(M5Stack CoreS3)で「最初の1サンプルで音が止まる」バグとして発覚しました。

## 6. `backend/esp32s3i2s`の内部実装

ESP32-S3のI2Sペリフェラルは、`rp2040i2s`(PIO)や`esp32s3pwm`(LEDC)のように「1サンプルずつ書き込む」モデルに乗りません。CPUから書けるFIFOデータレジスタが無く、**GDMA(汎用DMA)からの連続転送のみ**を前提にした設計だからです(ESP-IDFのI2Sドライバも常にDMAを使います)。以前はTinyGoの`device/esp`にESP32-S3用のGDMAレジスタ定義が無かったため未実装でしたが、**TinyGo 0.42で`esp.DMA`(GDMA)の定義が揃い**、実装できるようになりました。

I2S0とGDMAのレジスタ設定は、TinyGo Conf 2026バッジのファームウェア([tinygo-keeb/tinygo-conf-2026-badge](https://github.com/tinygo-keeb/tinygo-conf-2026-badge)の`firmware/i2s`、作者の許可を得て利用)から移したものです。こちらはESP-IDF v5.1の`hal/esp32s3/i2s_ll.h`、`gdma_ll.h`、`driver/i2s/i2s_std.c`の手順に合わせて書かれています。

- **フォーマット**: Philips標準I2S、16ビット、2チャネル(モノラルのサンプルを左右に複製)。スロット幅は16(BCLK=32fs、既定)か32(64fs)
- **クロック**: PLL 160MHzを分数分周してMCLK=fs×256を作り、そこからBCLKを作ります。16384Hzもほぼ正確に出せます(`esp32s3pwm`の1MHzタイマー÷61=16393Hzより正確)
- **DMAリング**: `BufferFrames`(既定64)フレームのバッファを`BufferCount`(既定4)個、ディスクリプタのリンクリストで輪にしてGDMAチャネル0に流し続けます
- **補充**: 各ディスクリプタに`suc_eof`を立てておき、バッファ1個を送り終えるたびに発生するGDMAの「out EOF」割り込み(CPU割り込みライン13)で、送り終えたバッファを`pull()`で埋め直します。そのバッファが再び再生されるのはリングを1周した後なので、(BufferCount−1)バッファ分の余裕があります。割り込みが遅れて複数バッファ分EOFが進んでいた場合も、`OUT_EOF_DES_ADDR`まで順に追いついて埋め直します
- **goroutineを使わない**: `esp32s3pwm`と同じ割り込み駆動なので、`rp2040i2s`のようにTinyGoの協調スケジューラに依存しません
- **音量**: `Output.SetVolume(0〜255)`で振幅を線形に調整できます(255が最大で既定値、128で約半分=-6dB)。8ビットのサンプルを16ビットのI2Sワードに広げる段階で掛けるため、音量を下げても分解能は落ちません

注意点として、補充が止まるとDMAリングは**古いバッファを繰り返し再生し続けます**(ブザーのような音になる)。このバックエンドは割り込みで常に補充するので通常は起きませんが、メインループが長時間`AudioHook()`を呼ばないと、エンジンのバッファが空になり最後のサンプルが繰り返されます。

実機ではM5Stack CoreS3の内蔵スピーカー(AW88298アンプ、BCK=G34/WS=G33/DOUT=G13)で確認しました。CoreS3ではアンプをI2Cで起こす処理(AW9523BのP0_2でリセット解除、AW88298のレジスタ設定。M5Unifiedの手順に準拠)が別途必要で、[`board/cores3`](../board/cores3)にまとめてあります(`cores3.NewSpeaker(audioRate)`でアンプの初期化と`esp32s3i2s`の生成を行う)。アンプの1.8V電源(AXP2101のALDO1)は既定でオンになっているため、そちらは読み取りで確認するだけです。MAX98357Aのような設定不要のアンプなら、ピンを指定するだけで使えます。

**アンプのレベル設定(重要)**: M5Unifiedはアンプを最大ゲイン(音量レジスタ0x0C=0x0064、VOL 0dB)で使いますが、M5Unified自身はアンプに振幅いっぱいの信号を送りません。一方mozziのスケッチは`From8Bit(osc.Next())`のように振幅いっぱいの信号を普通に出すため、最大ゲインのままだとアンプがクリップし、出力段が止まったり動いたりを繰り返して「ポップコーンのようなパチパチ音」になりました(実機で、AW88298のSYSSTのクリップビット(bit7)と出力段の動作ビット(bit8)を観測して確認)。アンプの音量段だけで-12dB下げてもクリップは残り(音量段より前の内部処理でクリップするため)、I2S入力で-6dB(I2SCTRL.INPLEV)+音量段で-12dBにしたところ、音量255でもクリップせず安定しました。`board/cores3`はこの設定にしています。[`tools/cores3-check`](../tools/cores3-check)で、クリップの有無を毎秒確認できます。

## 7. 新しいバックエンドを追加する

[05-extending.md](05-extending.md#新しいハードウェアバックエンドを追加する)を参照してください。
