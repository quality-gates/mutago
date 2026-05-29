# internal/models

`Report.HasCoverage` means "the coverage run happened." It does not mean any mutations were covered. This lets callers tell the difference between "coverage ran and found nothing uncovered" and "coverage was never run at all."
