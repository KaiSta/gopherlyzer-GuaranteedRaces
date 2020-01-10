package test;

public class Test24 extends Thread {
  static int x;
  static int a;
  static int y;
  static int z;
  static final Object m = new Object();
  static final Object n = new Object();

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1  
	synchronized(m) {
		System.out.println("l(m)");
		x = 1;
		System.out.println("w(x)");
		synchronized(n) {
			System.out.println("l(n)");
			System.out.println("u(n)");
		}
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(m) {
				System.out.println("l(n)");
				
				System.out.println("u(n)");
			}
			x = 2;
		}
   }


   public static void main(String args[]) throws Exception {
	final Test24 thread1 = new Test24();
        final T2 thread2 = new T2();

	thread1.start();
	thread2.start();

	thread1.join();
	thread2.join();

   }
}
