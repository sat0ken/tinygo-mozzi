# このTinyGo移植版のアーキテクチャ

[01-mozzi-original.md](01-mozzi-original.md)で説明した本家Mozziの概念が、このGo移植版でどう実装されているかを説明します。

## 1. フォルダ構成

```
mozzi/                      Goモジュールのルート(go.mod, go.sum)
├── fixmath/                 固定小数点演算
├── rand/                    xorshift96乱数
├── midi/                    MIDIノート→周波数変換
├── audioout/                オーディオサンプルの表現(Mono/Stereo)
├── oscil/                   ウェーブテーブルオシレータ、Phasor
├── line/                    線形補間(ジェネリック)
├── envelope/                ADSR, Ead, Smooth
├── eventdelay/               ノンブロッキングタイマー
├── filter/                  共振フィルタ
├── delay/                   ディレイライン(ジェネリック)
├── tables/                   波形テーブル(自動生成、59種類)
│   └── tablesdata/           テーブル本体(バイナリ、go:embed対象)
├── song/                     (独自追加)メロディシーケンサ
├── engine/                   コアスケジューラ(Sketch/Backendインターフェース)
├── backend/
│   ├── rp2040pwm/            RP2040: PWM出力バックエンド
│   ├── rp2040i2s/            RP2040: 外部I2S DAC/アンプ出力バックエンド
│   └── esp32s3pwm/           ESP32-S3: PWM出力バックエンド
├── tools/
│   └── gentables/             波形テーブルのCヘッダ→Go変換ツール
├── examples/                  実行可能なサンプル(12個)
├── docs/                      このドキュメント
├── README.md                  ユーザー向けクイックスタート/APIリファレンス
└── LICENSE.TXT / LICENCE.LGPL 本家と同じLGPL 2.1+
```

原則として、**本家Mozziの1ファイルが、TinyGo版のだいたい1パッケージに対応**するように設計しています(`Oscil.h`→`oscil`, `ADSR.h`+`Ead.h`+`Smooth.h`→`envelope`のようにまとめている場合もあります)。対応関係の一覧は[../README.md](../README.md#構成)の表、より詳細なものは[03-feature-coverage.md](03-feature-coverage.md)を参照してください。

## 2. C++→Goの主な変換方針

### 2.1 テンプレートパラメータ→フィールド/コンストラクタ引数

本家はC++テンプレートを多用しています(`Oscil<NUM_CELLS, RATE>`, `Line<T>`, `ADSR<RATE1, RATE2, T>`など)。これはAVR(数MHz、除算が遅い)でコンパイル時に定数畳み込みするための最適化です。

RP2040/ESP32-S3(数百MHz、32ビット)ではこの最適化の重要性が下がるため、**テンプレートパラメータの多くは通常のフィールド/コンストラクタ引数にしています**。例えば[`oscil.New(table []int8, updateRate uint32)`](../oscil/oscil.go)は、本家の`Oscil<NUM_CELLS, RATE>`のテンプレート引数を実行時の値として受け取ります。

トレードオフ: 本家より実行時のオーバーヘッド(除算1回程度)がわずかに増えますが、コードは大幅にシンプルになり、Go言語的にも自然になります。RP2040/ESP32-S3の処理能力なら問題になりません。

### 2.2 Goジェネリクスを使う場面

C++テンプレートがGoジェネリクスに素直に対応し、かつ実行時コストがほぼゼロで済む場合はジェネリクスを使っています。

- [`line.Line[T]`](../line/line.go): `T`は`~int8 | ~int16 | ... | ~float32 | ~float64`などの数値型制約。本家は型ごとに別クラス(`Line<unsigned char>`等)を特殊化していましたが、Goでは1つのジェネリック実装で足ります
- [`delay.AudioDelay[T]`](../delay/audiodelay.go): セルの型(`int8`, `int16`等)をジェネリックに

### 2.3 波形テーブル: `go:embed`でPROGMEM問題を解消

本家がPROGMEM/`pgm_read_byte()`という仕組みを必要とするのは、AVRがハーバードアーキテクチャで「プログラムメモリ(フラッシュ)」と「データメモリ(RAM)」が別のアドレス空間だからです。

**RP2040やESP32-S3のようなCortex-M/Xtensa系マイコンはこの制約がありません**(フラッシュがメモリ空間にマップされ、直接読み書きできる、いわゆるXIP)。そのため、TinyGo版では波形テーブルを単に`//go:embed`でバイナリとして埋め込んだ`[]int8`スライスとして扱うだけで、**RAM消費ゼロでフラッシュ常駐**を実現できています。

これは実測で確認済みです:59個のテーブル全てを(コンパイラが定数畳み込みできないよう)実行時に変化するインデックスで参照するテストコードをビルドしたところ、フラッシュ使用量はテーブルの合計サイズ分増えましたが、RAM使用量は全く増えませんでした。詳細は[../tables/doc.go](../tables/doc.go)と[`tools/gentables`](../tools/gentables)を参照してください。

**注意点**: `int16`のテーブル(2種類のみ)は、`go:embed`されたバイト列を直接`[]int16`として再解釈すると、TinyGoの一部ターゲットでアラインメント要件を満たさない可能性があるため、起動時に1回だけRAM上にコピーしています([`tables/conv.go`](../tables/conv.go)の`asInt16LE`)。これは例外的にRAMを消費しますが、対象テーブルが小さいため実用上問題になりません。

### 2.4 出力ビット幅を8ビット固定に単純化

本家は`MOZZI_AUDIO_BITS`という設定でプラットフォームごとに出力ビット幅(8/10/12/14ビット等)を変えられるようになっていますが、TinyGo版では**論理的な出力解像度を常に8ビットに固定**しています([`audioout.Bits = 8`](../audioout/audioout.go))。

理由: スケッチ側は常に`audioout.From8Bit(...)`のような「8ビットレンジの値」を返せばよく、実際のハードウェア解像度(PWMのTop値やI2Sのビット幅)へのスケーリングは**各バックエンドの責務**にしています。これにより、スケッチのコードがバックエンド非依存になり、出力先は[`board`](../board)パッケージがビルド時(TinyGoのターゲットとビルドタグ)に選びます。`examples/`以下のスケッチは、どれも1つのソースのまま全ボード向けにビルドできます(例: `-tags cores3`でM5Stack CoreS3、`-target=pico`でRaspberry Pi Pico)。

### 2.5 リングバッファの移植

本家[`CircularBuffer.h`](../../Mozzi/CircularBuffer.h)の「読み書き位置が一周したかを示すビット(mirroring bit)」というテクニックは、[`engine/ringbuffer.go`](../engine/ringbuffer.go)にほぼそのまま移植しています(256要素固定、`uint8`インデックスの自然なオーバーフローを利用)。

## 3. コアスケジューラ(`engine`パッケージ)

本家の`updateControl()`/`updateAudio()`のペアはGoでは`Sketch`インターフェースになっています。

```go
type Sketch interface {
    UpdateControl()
    UpdateAudio() audioout.Mono
}
```

ハードウェア出力は`Backend`インターフェースとして抽象化されています。

```go
type Backend interface {
    // pullを、サンプルレートの周期でoft呼び出し、1サンプルずつ取り出してハードウェアに書き込む。
    // 呼び出し元はタイマー割り込みでもgoroutineでもよい。
    Start(sampleRateHz uint32, pull func() audioout.Storage)
}
```

`engine.Start(sketch, audioRate, controlRate, backend)`を呼ぶと、`backend.Start()`が呼ばれてハードウェアの初期化が行われます。その後、ユーザーのコードは本家と同じように`for { engine.AudioHook() }`をひたすら回すだけです。`AudioHook()`の中身は本家の`audioHook()`とほぼ同じロジック(バッファに空きがあれば`UpdateControl`/`UpdateAudio`を呼んでバッファに書き込む)です。

### 3.1 重要な落とし穴: `runtime.Gosched()`

`Backend`の実装には2つのスタイルがあります。

1. **タイマー割り込み駆動**(`rp2040pwm`, `esp32s3pwm`): ハードウェアタイマーがCPUを強制的に中断させて`pull()`を呼ぶ。TinyGoのgoroutineスケジューリングとは無関係に、本物の割り込みとして動く
2. **goroutine駆動**(`rp2040i2s`): `Start()`内で`go`文によりgoroutineを起動し、そのgoroutineがハードウェアへのブロッキング書き込み(PIOのFIFOが埋まっていれば待つ)を繰り返すことで、結果的にリアルタイムのペースが保たれる

問題は、**TinyGoのgoroutineスケジューラは協調的(cooperative)**だという点です。あるgoroutineが`channel`操作や`runtime.Gosched()`のような明示的なyieldポイントを一度も通らないと、他のgoroutineには絶対に実行機会が回ってきません。

`for { engine.AudioHook() }`というメインループは、本来何もブロックしない関数を無限に呼び続けるだけなので、**goroutine駆動のバックエンドと組み合わせると、そのバックエンドのgoroutineが永遠に実行されず無音になる**という実際に起きたバグがありました(実機でRP2040-Zero + MAX98357Aをテストした際に発見)。

修正は`engine.AudioHook()`の最後で無条件に`runtime.Gosched()`を呼ぶことです([`engine/engine.go`](../engine/engine.go))。これにより、割り込み駆動のバックエンドには影響を与えず(他に走るgoroutineがなければコストはほぼゼロ)、goroutine駆動のバックエンドは正しく動くようになります。**今後goroutineを使う新しいバックエンドを書く場合も、この仕組みのおかげで特別な対応は不要です。**

## 4. 波形テーブル生成ツール(`tools/gentables`)

本家の`tables/*.h`は`CONSTTABLE_STORAGE(型) 名前[] = {...};`という規則的な形式です。[`tools/gentables/main.go`](../tools/gentables/main.go)はこれを正規表現で解析し、

1. 数値データをバイナリファイル(`.bin`)として書き出す
2. `//go:embed`でそのバイナリを埋め込むGoファイルを生成する

という機械的な変換を行います。このツールのおかげで、59個のテーブルすべてを手作業なしで移植できました。使い方は[../README.md](../README.md#波形テーブル)、拡張方法は[05-extending.md](05-extending.md)を参照してください。

## 5. 独自追加: `song`パッケージ

本家Mozziには存在しない、この移植版オリジナルの追加です。「特定の曲を鳴らしたい」というユースケース向けに、`[]song.Event{Note, Beats}`という単旋律のシーケンスを再生する小さなプレイヤーです。設計の詳細は[../song/song.go](../song/song.go)のコメント、実際の使い方は[../README.md](../README.md#メロディを鳴らす)を参照してください。

## 6. 典型的なサンプルの構造

```go
type sketch struct{ aSin *oscil.Oscil }             // Mozziスケッチのグローバル変数に相当
func (s *sketch) UpdateControl() {}                  // 本家のupdateControl()
func (s *sketch) UpdateAudio() audioout.Mono {        // 本家のupdateAudio()
    return audioout.From8Bit(int16(s.aSin.Next()))
}

func main() {                                          // 本家のsetup()+loop()
    s := &sketch{aSin: oscil.New(tables.Sin2048, audioRate)}
    s.aSin.SetFreq(440)
    out, _ := rp2040pwm.New(audioPin, carrierHz)       // ボード固有のバックエンドを選択
    engine.Start(s, audioRate, controlRate, out)        // 本家のstartMozzi()に相当
    for { engine.AudioHook() }                           // 本家のaudioHook()呼び出しループ
}
```

この対応関係を頭に入れておくと、本家の任意のサンプルスケッチを読んだときに「このGo版ではどう書けばいいか」がすぐわかるようになります。
