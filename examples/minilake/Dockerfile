# MiniLake 1.7.4. The patch preserves warehouse fields and exposes host metadata.
FROM ghcr.io/dmux/minilake:1.7.4@sha256:f6790408452a05c37e9685dc87bdf8b9823c26527053834eae24b6268bb0f8d1
COPY compatibility.patch /tmp/datatf-compatibility.patch
RUN patch --batch --fuzz=0 -p1 < /tmp/datatf-compatibility.patch
