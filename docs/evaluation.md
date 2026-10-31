# ScreenSift evaluation

Prepare at least 30 real images and at least 10 queries before measuring.
For each query record expected files in `ground_truth.csv`.

Precision = relevant returned files / all returned files.
Recall = relevant returned files / all expected files.

For performance compare 100, 500 and 1000 images and workers 1/2/4.
Record indexing time, average OCR time, search latency, CPU and memory.

Do not change the dataset or queries after inspecting results.
