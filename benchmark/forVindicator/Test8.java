package test;

public class Test8 extends Thread {
  static int x;
  static int y;
  static final Object m = new Object();

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
	x = 1;
	System.out.println("w(x)");
	synchronized(m) {		
		System.out.println("l(m)");
		x = 2;
		System.out.println("w(x)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(m) {
				System.out.println("l(m)");
				x = 3;
				System.out.println("w(x)");
				System.out.println("u(m)");
			}
			x = 4;
			System.out.println("w(x)");
		}
   }


   public static void main(String args[]) throws Exception {
	final Test8 thread1 = new Test8();
    final T2 thread2 = new T2();

	thread1.start();
	thread2.start();

	thread1.join();
	thread2.join();

   }
}
