#include <go.h>
#include <stdlib.h>
#include <string.h>
#include <threads.h>
#include <stdbool.h>
#include <stdint.h>

typedef struct sudog {
    struct sudog *next;
    void *elem;
    cnd_t cond;
} sudog;

typedef struct waitq {
    sudog *first;
    sudog *last;
} waitq;

typedef struct go_channel {
    mtx_t lock;
    size_t elemsize;
    size_t dataqsiz;
    size_t qcount;
    size_t recvx;
    size_t sendx;
    uint8_t *buf;
    waitq recvq;
    waitq sendq;
} go_channel;

static void enqueue(waitq *q, sudog *s) {
    s->next = NULL;
    if (q->last) {
        q->last->next = s;
    } else {
        q->first = s;
    }
    q->last = s;
}

static sudog *dequeue(waitq *q) {
    sudog *s = q->first;
    if (s) {
        q->first = s->next;
        if (q->first == NULL) {
            q->last = NULL;
        }
    }
    return s;
}

go_ch go_chan(go_ii elem_size, go_ii cap) {
    if (elem_size <= 0 || cap < 0) {
        return NULL;
    }
    go_channel *ch = malloc(sizeof(go_channel));
    if (ch == NULL) {
        return NULL;
    }
    ch->elemsize = (size_t)elem_size;
    ch->dataqsiz = (size_t)cap;
    ch->qcount = 0;
    ch->recvx = 0;
    ch->sendx = 0;
    if (mtx_init(&ch->lock, mtx_plain) != thrd_success) {
        free(ch);
        return NULL;
    }
    ch->recvq.first = NULL;
    ch->recvq.last = NULL;
    ch->sendq.first = NULL;
    ch->sendq.last = NULL;
    if (cap > 0) {
        ch->buf = malloc((size_t)cap * (size_t)elem_size);
        if (ch->buf == NULL) {
            mtx_destroy(&ch->lock);
            free(ch);
            return NULL;
        }
    } else {
        ch->buf = NULL;
    }
    return ch;
}

void go_send(go_ch c, go_ii size, const void* v) {
    go_channel *ch = c;
    if (ch == NULL || (size_t)size != ch->elemsize) {
        return; // Error: invalid channel or size mismatch
    }
    mtx_lock(&ch->lock);
    sudog *sg;
    if (ch->recvq.first != NULL) {
        sg = dequeue(&ch->recvq);
        memcpy(sg->elem, v, ch->elemsize);
        cnd_signal(&sg->cond);
        mtx_unlock(&ch->lock);
        return;
    } else if (ch->qcount < ch->dataqsiz) {
        uint8_t *qp = ch->buf + ch->sendx * ch->elemsize;
        memcpy(qp, v, ch->elemsize);
        ch->sendx = (ch->sendx + 1) % ch->dataqsiz;
        ch->qcount++;
        mtx_unlock(&ch->lock);
        return;
    }
    // Block
    sudog mysg;
    if (cnd_init(&mysg.cond) != thrd_success) {
        mtx_unlock(&ch->lock);
        return; // Error
    }
    mysg.elem = malloc(ch->elemsize);
    if (mysg.elem == NULL) {
        cnd_destroy(&mysg.cond);
        mtx_unlock(&ch->lock);
        return; // Error
    }
    memcpy(mysg.elem, v, ch->elemsize);
    enqueue(&ch->sendq, &mysg);
    cnd_wait(&mysg.cond, &ch->lock);
    free(mysg.elem);
    cnd_destroy(&mysg.cond);
    mtx_unlock(&ch->lock);
}

go_tf go_recv(go_ch c, go_ii size, void* v) {
    go_channel *ch = c;
    if (ch == NULL || (size_t)size != ch->elemsize) {
        return false; // Error: invalid channel or size mismatch
    }
    mtx_lock(&ch->lock);
    sudog *sg;
    if (ch->qcount > 0) {
        uint8_t *qp = ch->buf + ch->recvx * ch->elemsize;
        memcpy(v, qp, ch->elemsize);
        bool was_full = (ch->qcount == ch->dataqsiz);
        if (was_full && ch->sendq.first != NULL) {
            sg = dequeue(&ch->sendq);
            memcpy(qp, sg->elem, ch->elemsize);
            free(sg->elem);
            cnd_signal(&sg->cond);
            ch->recvx = (ch->recvx + 1) % ch->dataqsiz;
            ch->sendx = ch->recvx;
            // qcount unchanged
        } else {
            ch->recvx = (ch->recvx + 1) % ch->dataqsiz;
            ch->qcount--;
        }
        mtx_unlock(&ch->lock);
        return true;
    } else if (ch->sendq.first != NULL) {
        sg = dequeue(&ch->sendq);
        memcpy(v, sg->elem, ch->elemsize);
        free(sg->elem);
        cnd_signal(&sg->cond);
        mtx_unlock(&ch->lock);
        return true;
    }
    // Block
    sudog mysg;
    if (cnd_init(&mysg.cond) != thrd_success) {
        mtx_unlock(&ch->lock);
        return false; // Error
    }
    mysg.elem = v;
    enqueue(&ch->recvq, &mysg);
    cnd_wait(&mysg.cond, &ch->lock);
    cnd_destroy(&mysg.cond);
    mtx_unlock(&ch->lock);
    return true;
}
