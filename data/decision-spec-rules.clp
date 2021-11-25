(deftemplate estimated-transfer
    (multislot from-to (type SYMBOL))
    (slot value (type FLOAT))
    (slot CF (type FLOAT)))
    
(deftemplate estimated-exec
    (slot at (type SYMBOL))
    (slot value (type FLOAT))
    (slot CF (type FLOAT)))    

(deftemplate estimated-waiting
    (slot at (type SYMBOL))
    (slot value (type FLOAT))
    (slot CF (type FLOAT)))

(defglobal ?*slack-macmini* = 0)
(defglobal ?*slack-server* = 0)
(defglobal ?*slack-local* = 0)


;***********************Regla macmini
(defrule execution-macmini 
	(declare (salience ?*slack-macmini*))
    (estimated-transfer 
						(from-to ?a inter)
                        (value ?v-txon)
                        (CF ?CF-txon))
    (estimated-waiting  
						(at inter)
                        (value ?v-wait)
                        (CF ?CF-wait))
    (estimated-exec     
						(at inter)
                        (value ?v-tex)
                        (CF ?CF-tex))
	(deadline ?dl)
	(test (neq ?a inter))
	(pointer chunk-chan ?pointer ?id)
    ?ef <-      (fire executionr&:(bind ?*slack-macmini* (integer (* 100 (min ?CF-txon ?CF-wait ?CF-tex)))))
	(test (<= (+ ?v-txon ?v-wait ?v-tex) ?dl)) ; ?v-txon ?v-wait ?v-tex <= deadline (?dl)?
	(test (> (min ?CF-txon ?CF-wait ?CF-tex) 0.2)) ; es aceptable la invertidumbre (incertidumbre > 0.2)?
=>
	(retract ?ef)
	(process-at-macmini ?pointer ?id))
   

;***********************Regla server


(defrule execution-server 
	(declare (salience ?*slack-server*))
    (estimated-transfer 
						(from-to ?a server)
                        (value ?v-txon)
                        (CF ?CF-txon))
    (estimated-waiting  
						(at server)
                        (value ?v-wait)
                        (CF ?CF-wait))
    (estimated-exec     
						(at server)
                        (value ?v-tex)
                        (CF ?CF-tex))
	(deadline ?dl)
	(test (neq ?a server))
	(pointer chunk-chan ?pointer ?id)
    ?ef <-      (fire executionr&:(bind ?*slack-server* (integer (* 100 (min ?CF-txon ?CF-wait ?CF-tex)))))
    (test (<= (+ ?v-txon ?v-wait ?v-tex) ?dl)) ; ?CF-txon ?CF-wait ?CF-tex <= deadline (200)?
	(test (> (min ?CF-txon ?CF-wait ?CF-tex) 0.2)) ; es aceptable la invertidumbre (incertidumbre > 0.2)?
=>
   (retract ?ef)
   (process-at-server ?pointer ?id))
   
   

(defrule execution-local
	(declare (salience ?*slack-local*))
	(estimated-transfer 
						(from-to ?local ?local)
                        (value ?v-txon)
                        (CF ?CF-txon))
    (estimated-waiting  
						(at ?local)
                        (value ?v-wait)
                        (CF ?CF-wait))
    (estimated-exec     
						(at ?local)
                        (value ?v-tex)
                        (CF ?CF-tex))
	(deadline ?dl)
	(pointer chunk-chan ?pointer ?id)
	?ef <-      (fire executionr&:(bind ?*slack-local* (integer (* 100 (min ?CF-txon ?CF-wait ?CF-tex)))))
	(test (<= (+ ?v-txon ?v-wait ?v-tex) ?dl))
	(test (> (min ?CF-txon ?CF-wait ?CF-tex) 0.2))
=>
   (retract ?ef)
   (process-at-local ?pointer ?id))
   
(defrule no_execution
	(declare (salience 18))
	?ef <- (fire executionr)
	(CF-at ?place ?)
	(pointer chunk-chan ?pointer ?id)
=>
	(retract ?ef)
	(no-action ?pointer ?id ?place))
   
(defrule total_CF
	(estimated-transfer
		(from-to ? ?local)
        (value ?v-txon)
        (CF ?CF-txon))
    (estimated-waiting  
		(at ?local)
        (value ?v-wait)
        (CF ?CF-wait))
    (estimated-exec     
		(at ?local)
        (value ?v-tex)
        (CF ?CF-tex))
=>		
	(assert (CF-at ?local (+ ?CF-txon ?CF-wait ?CF-tex))))

(defrule min_value
	(declare (salience 19))
	?ef <-	(CF-at ?a ?t)
	(CF-at ?b ?t2)
	(test (neq ?a ?b))
	(test (>= ?t2 ?t))
=>
	(retract ?ef))
	
;***********************conjunto de reglas y funciones para los hechos de alto nivel
(deffunction lineal-regresion
	;duración, pendiente, ordenada en el origen
	(?x ?m ?n)
	;calculo
	(+ (* ?x ?m) ?n))
	
(deffunction tx_calculator
	;tamaño, velocidad del enlace
	(?size ?bandwidth ?chunksize)
	;calculo
	(float (/ (+ ?size (* 41 (/ ?size ?chunksize))) ?bandwidth)))

(defrule calc_transfer
	?rule <- (transfer-to-calc ?origen ?destino ?size ?bandwidth ?chunksize ?CF)
	(test (neq ?origen ?destino))
=>
	(retract ?rule)
	(assert (estimated-transfer (from-to ?origen ?destino) (value (tx_calculator ?size ?bandwidth ?chunksize)) (CF ?CF))))
	
	
(defrule local_transfer
	?rule <- (transfer-to-calc ?origen ?origen ?size ?bandwidth ?chunksize ?CF)
=>
	(retract ?rule)
	(assert (estimated-transfer (from-to ?origen ?origen) (value 0.0) (CF 1.0))))
	


(defrule calc_waiting
	?rule <- (waiting-to-calc ?destino ?cola ?tiempo-medio ?CF)
=>	
	(retract ?rule)
	(assert (estimated-waiting (at ?destino) (value (* ?cola ?tiempo-medio)) (CF ?CF))))

(defrule calc_exec
	?rule <- (exec-to-calc ?destino ?x ?m ?n)
=>
	(retract ?rule)
	(assert (estimated-exec (at ?destino) (value (lineal-regresion ?x ?m ?n)) (CF 0.8))))