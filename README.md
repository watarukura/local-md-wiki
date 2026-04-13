# lwm

ローカル専用のMarkdown Wikiです。(lwm: **l**ocal **w**iki.**m**d)
認証機能はないので、公開での使用はしないでください。
ブラウザから `.md` ファイルを編集し、backlinks / 2-hop links を確認できます。

---

## 特徴

- Markdown ファイルがそのまま実体
- ブラウザから編集・保存
- backlinks / 2-hop links 表示
- 画像の貼り付け対応（clipboard → 自動アップロード）

---

## 動作要件

- Go 1.21+
- aquaproj/aqua/aqua

---

## セットアップ

```bash
go mod tidy
```

---

## 起動

```bash
go run .
```

または Makefile を使用してビルド・実行:

```bash
make build
./mdwiki
```

アクセス:

```
http://localhost:3000
```

---

## ディレクトリ構成

```text
.
├── main.go            # Goバックエンド
├── public/
│   ├── index.html
│   ├── app.js
│   ├── style.css
│   └── uploads/       # 画像保存先
└── pages/             # Markdownファイル
    ├── Home.md
    ├── AWS.md
    └── ECS.md
```

---

## 使い方

### ページの作成

- 左ペインの `New` ボタン
- 名前を入力（`.md` は省略可）

### 編集

- `Edit` → 編集 → `Save`

### 内部リンク

Markdown link を使用します:

```md
[AWS](AWS.md)
[ECS](infra/ECS.md)
```

---

## 画像貼り付け

エディタに画像を **貼り付け（Ctrl+V / Cmd+V）** すると:

- `/public/uploads/` に保存
- Markdown が自動挿入される

```md
![pasted image](/static/uploads/xxxxx.png)
```

対応形式:

- png / jpeg / gif / webp

---

## backlinks / 2-hop links

### backlinks

現在のページを参照しているページ一覧

### 2-hop links

「同じページにリンクしている他のページ」

例:

```
A -> X
B -> X
```

→ A から見たとき B が 2-hop link

---

## コードブロック

```js
console.log("hello");
```

- highlight.js によるシンタックスハイライト
- 言語未指定でも自動判定

---

## 開発用コマンド

Makefile を使用して開発に必要なコマンドを実行できます。

```bash
make build   # バイナリをビルド
make test    # テストを実行
make lint    # 静的解析を実行 (golangci-lint)
make format  # コードを整形 (go fmt)
make clean   # ビルド生成物を削除
make help    # 利用可能なコマンドを表示
```

---

## 制限事項

- 認証なし
- 同時編集なし
- 検索なし
- 画像の削除・整理なし
- 大規模データ非対応

---

## ライセンス

MIT
