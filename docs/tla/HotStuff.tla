---- MODULE HotStuff ----
EXTENDS Naturals, FiniteSets, TLC

CONSTANTS N, F, MaxHeight

ASSUME N > 3 * F

PhasePrepare == 0
PhasePreCommit == 1
PhaseCommit == 2
PhaseDecide == 3

Replicas == 0..(N-1)
Quorum == N - F

VARIABLES height, view, phase, decided, decidedHash, lockedView, messages, maxDecided

vars == <<height, view, phase, decided, decidedHash, lockedView, messages, maxDecided>>

Init ==
    /\ height = [r \in Replicas |-> 1]
    /\ view = [r \in Replicas |-> 0]
    /\ phase = [r \in Replicas |-> PhasePrepare]
    /\ decided = [r \in Replicas |-> FALSE]
    /\ decidedHash = [r \in Replicas |-> 0]
    /\ lockedView = [r \in Replicas |-> 0]
    /\ messages = {}
    /\ maxDecided = 0

Leader(h, v) == (h + v) % N

CanDecide(r) ==
    LET sameHeight == {s \in Replicas : (height[s] = height[r]) /\ decided[s]}
    IN Cardinality(sameHeight) >= Quorum

Advance(r) ==
    /\ decided[r] = TRUE
    /\ height' = [height EXCEPT ![r] = height[r] + 1]
    /\ view' = [view EXCEPT ![r] = 0]
    /\ phase' = [phase EXCEPT ![r] = PhasePrepare]
    /\ decided' = [decided EXCEPT ![r] = FALSE]
    /\ decidedHash' = [decidedHash EXCEPT ![r] = 0]
    /\ lockedView' = [lockedView EXCEPT ![r] = 0]
    /\ UNCHANGED <<messages, maxDecided>>

Propose(r) ==
    /\ Leader(height[r], view[r]) = r
    /\ phase[r] = PhasePrepare
    /\ decided[r] = FALSE
    /\ messages' = messages \cup {[h |-> height[r], v |-> view[r],
        sender |-> r, typ |-> "proposal", bh |-> height[r]]}
    /\ UNCHANGED <<height, view, phase, decided, decidedHash, lockedView, maxDecided>>

VotePrepare(r) ==
    /\ phase[r] = PhasePrepare
    /\ decided[r] = FALSE
    /\ \E m \in messages : (m.typ = "proposal") /\ (m.h = height[r]) /\ (m.v = view[r]) /\ (Leader(m.h, m.v) = m.sender)
    /\ messages' = messages \cup {[h |-> height[r], v |-> view[r],
        sender |-> r, typ |-> "vote_prepare", bh |-> height[r]]}
    /\ phase' = [phase EXCEPT ![r] = PhasePreCommit]
    /\ UNCHANGED <<height, view, decided, decidedHash, lockedView, maxDecided>>

CollectPreCommit(r) ==
    LET vms == {m \in messages : (m.typ = "vote_prepare") /\ (m.h = height[r]) /\ (m.v = view[r])}
        voters == {m.sender : m \in vms}
    IN /\ phase[r] = PhasePreCommit
       /\ decided[r] = FALSE
       /\ Cardinality(voters) >= Quorum
       /\ phase' = [phase EXCEPT ![r] = PhaseCommit]
       /\ messages' = messages \cup {[h |-> height[r], v |-> view[r],
           sender |-> r, typ |-> "vote_precommit", bh |-> height[r]]}
       /\ lockedView' = [lockedView EXCEPT ![r] = view[r]]
       /\ UNCHANGED <<height, view, decided, decidedHash, maxDecided>>

CollectCommit(r) ==
    LET vms == {m \in messages : (m.typ = "vote_precommit") /\ (m.h = height[r]) /\ (m.v = view[r])}
        voters == {m.sender : m \in vms}
    IN /\ phase[r] = PhaseCommit
       /\ decided[r] = FALSE
       /\ Cardinality(voters) >= Quorum
       /\ phase' = [phase EXCEPT ![r] = PhaseDecide]
       /\ messages' = messages \cup {[h |-> height[r], v |-> view[r],
           sender |-> r, typ |-> "vote_commit", bh |-> height[r]]}
       /\ UNCHANGED <<height, view, decided, decidedHash, lockedView, maxDecided>>

Decide(r) ==
    LET vms == {m \in messages : (m.typ = "vote_commit") /\ (m.h = height[r]) /\ (m.v = view[r])}
        voters == {m.sender : m \in vms}
    IN /\ phase[r] = PhaseDecide
       /\ decided[r] = FALSE
       /\ Cardinality(voters) >= Quorum
       /\ decided' = [decided EXCEPT ![r] = TRUE]
       /\ decidedHash' = [decidedHash EXCEPT ![r] = height[r]]
       /\ maxDecided' = IF height[r] > maxDecided THEN height[r] ELSE maxDecided
       /\ UNCHANGED <<height, view, phase, lockedView, messages>>

Timeout(r) ==
    /\ decided[r] = FALSE
    /\ view' = [view EXCEPT ![r] = view[r] + 1]
    /\ phase' = [phase EXCEPT ![r] = PhasePrepare]
    /\ UNCHANGED <<height, decided, decidedHash, lockedView, messages, maxDecided>>

Next ==
    \/ \E r \in Replicas : Propose(r)
    \/ \E r \in Replicas : VotePrepare(r)
    \/ \E r \in Replicas : CollectPreCommit(r)
    \/ \E r \in Replicas : CollectCommit(r)
    \/ \E r \in Replicas : Decide(r)

Agreement ==
    \A r1, r2 \in Replicas :
        (decided[r1] /\ decided[r2] /\ height[r1] = height[r2])
        => decidedHash[r1] = decidedHash[r2]

PhaseValid ==
    \A r \in Replicas : phase[r] \in {PhasePrepare, PhasePreCommit, PhaseCommit, PhaseDecide}

Safety == /\ Agreement /\ PhaseValid

MessagesLimited == Cardinality(messages) < 6

====
