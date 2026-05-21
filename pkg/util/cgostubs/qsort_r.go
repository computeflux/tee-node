// Package cgostubs provides C symbols missing from EGo enclave musl libc.
//
// EGo (Edgeless Go) enclave uses a musl-based libc provided by Open Enclave.
// Some glibc extensions (e.g. qsort_r) are absent from that libc, causing
// enclave load failures when CGO code (such as DataDog/zstd) references them.
//
// Import this package (usually anonymously in main) to ensure the stubs are
// linked into the final binary.
package cgostubs

/*
#include <stdlib.h>

// GNU-style qsort_r signature used by glibc and zstd:
//   void qsort_r(void *base, size_t nmemb, size_t size,
//                int (*cmp)(const void *, const void *, void *),
//                void *arg);
//
// musl does not provide qsort_r, so we implement it on top of the standard
// qsort(3) using thread-local storage to pass the comparator and its context.

static __thread void       *qsort_r_tls_arg;
static __thread int       (*qsort_r_tls_cmp)(const void *, const void *, void *);

static int qsort_r_tls_wrapper(const void *a, const void *b) {
	return qsort_r_tls_cmp(a, b, qsort_r_tls_arg);
}

void qsort_r(void *base, size_t nmemb, size_t size,
		     int (*compar)(const void *, const void *, void *),
		     void *arg) {
	qsort_r_tls_arg = arg;
	qsort_r_tls_cmp = compar;
	qsort(base, nmemb, size, qsort_r_tls_wrapper);
}
*/
import "C"
