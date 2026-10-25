# 保守・拡張の実践ガイド

このドキュメントは、実際に手を動かして機能を追加/修正する際の具体的な手順をまとめたものです。

## 0. 開発環境

- [TinyGo](https://tinygo.org/) 0.41以降(このプロジェクトは0.41.1で開発)。`tinygo version`で確認
- 標準の`go` (TinyGoが内部で使うホストGo)。`go version`で確認
- 作業ディレクトリは`mozzi/`(このリポジトリの`mozzi`フォルダ)。`go.mod`がここにあります

### ビルド確認コマンド一覧

```sh
cd mozzi

# ハードウェア非依存パッケージ(fixmath, rand, midi, audioout, oscil, line,
# envelope, filter, delay, engine, eventdelay, metronome, rollingaverage,
# samplehuffman, samples, tables, song)は普通のgoでビルド・検証できる
# (高速で、ハードウェア固有の依存がないため)
PKGS="./fixmath/... ./rand/... ./midi/... ./audioout/... ./oscil/... ./line/...
      ./envelope/... ./filter/... ./delay/... ./engine/... ./eventdelay/...
      ./metronome/... ./rollingaverage/... ./samplehuffman/... ./samples/...
      ./tables/... ./song/... ./tools/gentables ./tools/genhuffman"
go build $PKGS
go vet   $PKGS
go test  ./line ./rand
gofmt -l .   # 出力が空ならフォーマット済み

# backend/*、board、analog、examples/* はハードウェア固有の依存(machine, device/esp等)
# があるため、必ずtinygo buildで、対象ボードのtargetを指定してビルドする
# (ボードの選び方はboardパッケージのドキュメント参照)
tinygo build -target=pico                                    -size short -o /tmp/x.uf2 ./examples/01.Basics/Sinewave
tinygo build -target=waveshare-rp2040-zero -tags rp2040i2s   -size short -o /tmp/x.uf2 ./examples/01.Basics/Sinewave
tinygo build -target=esp32s3-generic       -tags esp32s3pwm  -size short -o /tmp/x.bin ./examples/01.Basics/Sinewave
tinygo build -target=esp32s3-generic       -tags cores3      -size short -o /tmp/x.bin ./examples/01.Basics/Sinewave
```

**注意**: `backend/*`や`examples/*`のファイルをエディタ(gopls)で開くと`could not import machine`のようなエラーが出ますが、これは無視して構いません。TinyGoの`machine`/`device/*`パッケージは通常のGoツールチェインからは見えない(TinyGo独自のGOROOTにある)ためで、`tinygo build`では正常にビルドできます。

## 1. 本家Mozziから新しい機能を移植する

典型的な流れ(このセッションで実際に繰り返した手順です):

1. **本家のヘッダファイルを読む**: `Mozzi/<対象>.h`をよく読む。特に、テンプレートパラメータが何のためにあるか、内部でどんな固定小数点型(`Q*n*`)を使っているか、を把握する
2. **対応するGoパッケージ/ファイルの場所を決める**: 既存のパッケージ構成([02-port-architecture.md](02-port-architecture.md#1-フォルダ構成))に合わせる。1ファイル=1パッケージが基本方針
3. **移植方針を決める**:
   - テンプレートパラメータ → コンストラクタ引数/フィールドにするのが基本
   - 型ごとの特殊化(C++の`template<> class Foo<int8_t>`等) → 複数の具体型を持つか、Goジェネリクスで1本化するか判断する。ジェネリクスにする場合は、型ごとの計算式の違い(ビットシフト量など)が本当に共通化できるか要注意
   - 内部で使われる`Q*n*`型は`fixmath`パッケージの対応する型を使う
4. **忠実に移植する。ただし正しさ・安全性を優先する変更は許容**: このプロジェクトでは「配列外読み出し(C++では未定義動作だが実害がなかった)をクランプに変える」「複数の異なる整数幅を使うC++コードを、単一の広い型(int32/int64)に統一する」といった変更を、コメントで理由を明記した上で行っています。詳細は[../README.md](../README.md#忠実さについて)の「忠実さについて」を参照
5. **`go build`/`go vet`で確認**(ハードウェア非依存パッケージの場合)
6. **必要ならサンプルを1つ作って`tinygo build`で実機ターゲット向けにも確認**
7. **このドキュメント(`docs/03-feature-coverage.md`)を更新する**: 移植した項目を「❌ 未移植」から「✅ 移植済み」に変更し、対応するGoファイルへのリンクを追加

## 2. 新しい波形テーブルを追加する

本家形式(`CONSTTABLE_STORAGE(型) 名前[] = {...};`)のヘッダファイルであれば、[`tools/gentables`](../tools/gentables)で自動変換できます。

```sh
cd mozzi
go run ./tools/gentables <入力ヘッダのあるディレクトリ> <出力先パッケージディレクトリ>
# 例: BandLimitedテーブルを生成する場合
go run ./tools/gentables ../Mozzi/tables/BandLimited_SAW tables/bandlimited
```

生成されるファイル名・変数名の規則は[`tools/gentables/main.go`](../tools/gentables/main.go)のコメントを参照してください(配列名から`_DATA`サフィックスを除き、CamelCaseに変換)。

自作の波形データ(本家の形式に従っていない)を追加したい場合は、既存の生成済みファイル(例: [`tables/sin2048_int8.go`](../tables/sin2048_int8.go))をコピーして、`.bin`ファイルとGoファイルを手動で用意しても構いません。

## 3. 新しいハードウェアバックエンドを追加する

1. `engine.Backend`インターフェース(`Start(sampleRateHz uint32, pull func() audioout.Storage)`)を満たす型を実装する
2. 「割り込み駆動」と「goroutine駆動」のどちらが対象ハードウェアに合うか判断する:
   - ハードウェアタイマー+`interrupt.New()`が使える → 割り込み駆動([`backend/rp2040pwm`](../backend/rp2040pwm)または[`backend/esp32s3pwm`](../backend/esp32s3pwm)をテンプレートに)
   - PIOのような「ハードウェアが自然にペースを作ってくれる」ブロッキングAPIがある → goroutine駆動([`backend/rp2040i2s`](../backend/rp2040i2s)をテンプレートに)。**この場合`engine.AudioHook()`が`runtime.Gosched()`を呼ぶおかげで、呼び出し側の対応は不要**([02](02-port-architecture.md#31-重要な落とし穴-runtimegosched)参照)
3. **TinyGoのランタイム/machineパッケージが同じタイマー/割り込みラインを既に使っていないか確認する**([04-hardware-backends.md](04-hardware-backends.md)の「教訓」参照)。確認方法:
   ```sh
   TINYGOROOT=$(tinygo env TINYGOROOT)
   grep -rn "対象のレジスタ名や割り込み定数" "$TINYGOROOT/src/runtime" "$TINYGOROOT/src/machine"
   ```
4. サンプルの出力レベルは、`audioout.Storage`(論理8ビット、-128〜127相当)をハードウェアの実際の解像度にスケーリングする(`rp2040pwm`/`esp32s3pwm`の`isr()`内のスケーリング処理を参照)
5. [`board`](../board)パッケージに`board_<名前>.go`を追加し、新しいビルドタグ(またはtarget)で`NewOutput`・`A0`・`Button`・`applyVolume`を定義する。これで既存の全サンプルがそのボード向けにビルドできるようになる。`tinygo build -target=<target> -tags <タグ> ./examples/01.Basics/Sinewave`で確認
6. [../README.md](../README.md#対応ボード)の対応表と[04-hardware-backends.md](04-hardware-backends.md)を更新する

## 4. 新しいサンプルを作る

- 通常のシンセ系サンプル: [`examples/01.Basics/Skeleton`](../examples/01.Basics/Skeleton)をコピーして`UpdateControl`/`UpdateAudio`を実装する。出力は`board.NewOutput(audioRate)`で得るので、ボード依存のコードは書かない
- 本家のサンプルを移植する場合は、本家と同じパス(例: `../Mozzi/examples/03.Sensors/Volume_Knob` → `examples/03.Sensors/Volume_Knob`)に置く
- メロディ再生系サンプル: [`examples/TinyGo_Extras/fanfare`](../examples/TinyGo_Extras/fanfare)をコピーする。`song.Event`配列の作り方は[../README.md](../README.md#メロディを鳴らす)を参照

## 5. 曲の採譜(メロディ書き起こし)を行う場合

[`examples/TinyGo_Extras/fanfare`](../examples/TinyGo_Extras/fanfare)は、実際に音声ファイルから`ffmpeg`+自作の簡易ピッチ追跡スクリプトで採譜した例です。同じ手法を別の曲に使う場合の流れ:

1. `ffmpeg -i <動画> -vn -ac 1 -ar 22050 -f wav audio_mono.wav`で音声を抽出
2. numpyだけで実装した自己相関ベースのピッチ検出(scipy/librosa/aubioは本環境に無かったため自作した)で、時間ごとの周波数を推定
3. エネルギー(音量)の変化を見て、**同じ音程が続く区間でも実際には吹き直し/弾き直しがある**ことを検出する(ピッチだけでは同一音程の連続再アタックを検出できないため、別途エネルギーの谷を見る必要がある)
4. `song.Event`の`Beats`は`song.Player`の`bpm`に対する相対値なので、テンポを変えたい場合は`bpm`定数を1箇所変えるだけでよい(個々の`Beats`値は変更不要)

**単旋律の楽器(トランペット等)ではこの手法はよく機能しましたが、ピアノのような和音・ペダルを伴う音源では、どの瞬間也どの音が主旋律か判別できず、精度が大きく落ちました**(実例: [`examples/TinyGo_Extras/furuhata_op`](../examples/TinyGo_Extras/furuhata_op)のコメント参照)。ピアノ等の複雑な音源を高精度に採譜したい場合は、この単純な自己相関法ではなく、より高度な(ポリフォニック対応の)ピッチ検出手法が必要になります。

**採譜結果の確度を明示する**: 実測できた部分と、推定/未確認の部分を、コード中のコメントで明確に区別すること。「確信のない転記を、確信があるかのように提示しない」というのが、このプロジェクト全体を通しての方針です。

## 6. トラブルシューティング

| 症状 | よくある原因 |
|---|---|
| 音が全く出ない(goroutine駆動バックエンド) | `engine.AudioHook()`が`runtime.Gosched()`を呼んでいるか確認(呼んでいなければメインループがCPUを独占し、出力用goroutineが動かない) |
| `handlers for interrupt N in multiple packages` | 選んだ割り込み番号/タイマーがTinyGoのruntime/machineパッケージと衝突している。別の番号/タイマーを試す([04-hardware-backends.md](04-hardware-backends.md)参照) |
| `could not import machine`等(エディタ上のみ) | gopls(通常のGoツールチェイン)の制限。`tinygo build`で実際にビルドが通るか確認すればよい |
| フラッシュ容量オーバー | 大きな波形テーブル(特にBandLimited系)を必要以上にimportしていないか確認。使われていないテーブルはリンカが自動的に除外するはずなので、実際に参照しているテーブルの合計サイズを見直す |
