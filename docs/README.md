# mozzi(TinyGo版)開発者ドキュメント

このフォルダは、`mozzi/`(TinyGo版Mozziライブラリ)を**今後保守・機能拡張していく人向け**のドキュメント集です。トップレベルの[`../README.md`](../README.md)がユーザー向けのクイックスタート/APIリファレンスだとすると、こちらは「なぜこう作ったか」「どう保守するか」を説明する開発者向けの資料です。

## 読む順番

新しくこのプロジェクトに関わる場合は、上から順に読むことを推奨します。

1. **[01-mozzi-original.md](01-mozzi-original.md)** — 移植元の本家Mozzi(Arduino C++ライブラリ)の解説。アーキテクチャ・主要概念・用語集。**本家Mozziに詳しくない場合はここから読んでください。**
2. **[02-port-architecture.md](02-port-architecture.md)** — このTinyGo移植版の設計。フォルダ構成、C++→Goの変換方針、なぜそう設計したか。
3. **[03-feature-coverage.md](03-feature-coverage.md)** — 本家Mozziのどのファイル/機能を移植し、何を移植していないかの詳細な対応表。
4. **[04-hardware-backends.md](04-hardware-backends.md)** — 対応マイコン(RP2040, ESP32-S3)と、各バックエンドの内部実装(タイマー割り込み・PIO・レジスタ操作など)の詳細。
5. **[05-extending.md](05-extending.md)** — 新しい機能を移植する/新しいボードに対応する/新しいサンプルを作る、といった実際の作業手順。

## この移植の一言まとめ

- 本家Mozziは「~60個のコアヘッダ + ~60個の波形テーブル + ~15種類のハードウェアバックエンド + 100以上のサンプル」という大規模なArduinoライブラリです。
- このTinyGo版は、**コア機能(オシレータ・エンベロープ・フィルタ・ディレイ・固定小数点演算などの中枢部分)を忠実に移植**し、**波形テーブルは全59種を機械的に変換**、**対応ハードウェアはRP2040とESP32-S3の4バックエンド**に絞った「実用的なコア移植」です。全ファイルの逐語訳ではありません。
- 独自追加として`mozzi/song`(メロディ再生用の小さなシーケンサ)があります。これは本家Mozziには存在しない、この移植版オリジナルの機能です。

## 現在の状態(要点)

- パッケージ数: 中核パッケージ(engine, oscil, envelope, filter, delay, metronome, samplehuffman など) + 4個のハードウェアバックエンド + ボード選択パッケージ(`board`、`board/cores3`) + ツール(`tools/gentables`、`tools/genhuffman`、`tools/cores3-original`)
- サンプル数: 24個。`examples/`は本家Mozziの`examples/`と同じフォルダ構成(本家の`01.Basics`・`02.Control`は全部移植済み。他に`06.Synthesis/FMsynth`など4個と、独自の`TinyGo_Extras/`に4個)。どのサンプルも1つのソースのまま、ビルドタグでボードを選んでビルドする(例: `-tags cores3`)
- 実機動作確認: `backend/rp2040i2s`はWaveshare RP2040-Zero + MAX98357Aで音出し確認済み。`esp32s3pwm`はM5Stack CoreS3系の実機で、チップ内の自己診断(PWMピンのレベルをGPIO入力で読み戻す方式)により確認済みですが、音出しはまだです。`esp32s3i2s`はM5Stack CoreS3の内蔵スピーカー(AW88298アンプ)で音出し確認済みです。`rp2040pwm`は`tinygo build`でのコンパイル確認のみで、実機未検証です。
- ビルド確認コマンド: `cd mozzi && tinygo build -target=<target> [-tags <ボード>] -size short ./examples/<カテゴリ>/<名前>`(例: `tinygo build -target=esp32s3-generic -tags cores3 -size short ./examples/01.Basics/Sinewave`)

詳細はそれぞれのドキュメントを参照してください。
