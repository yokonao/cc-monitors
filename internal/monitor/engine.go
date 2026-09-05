package monitor

// Claude Code Monitor の Engine 部分
// https://github.com/octokit/octopoller.rb
// ↑のような poller を立てて、任意ロジックを渡して polling できる
// 任意ロジックは event 列と polling 継続するかどうかをかえす
// event 列は emit して polling をつづけたりつづけなかったり
// report event がかえったらそれを emit する
