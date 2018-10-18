# Benchmarks
With the goal of making this codebase small a somewhat stange persistence model
was chosen. Several places in the code we write a serizalized Map to the disk.
This is a simple and convenient way to store data between runs, but is obviously
less efficient than a real database. To address this, I've run some Benchmarks.
You can run them yourself (under resource/metadata tests), however I'm putting
a copy of the data here for quick reference.

Benchmark writes loop on doing one of writes
BatchWrite fills the map and then writes it all at once
BatchLoad loads a map that has already been filled
The numbers after the test name indicate the number of elements in the map
BenchmarkWrite1-8                   1000           1549002 ns/op
BenchmarkWrite10-8                   100          19359984 ns/op
BenchmarkWrite100-8                    3         384999766 ns/op
BenchmarkWrite500-8                    1        2898997700 ns/op
BenchmarkWrite5000-8                   1        47505503100 ns/op
BenchmarkBatchWrite100000-8           30          34133303 ns/op
BenchmarkBatchLoad100000-8            50          23619956 ns/op

These results indicate a pretty linear increase in write time. This means
that larger writes don't take that much longer, the dominant factor is the number
of writes. To me this means the current scheme isn't too bad, and so I intend to
leave it in place until it becomes an issue. Also, the data model is simple enough
that moving to github.com/boltdb/bolt would be pretty easy.
